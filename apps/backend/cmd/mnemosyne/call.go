package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aiyu-ayaan/mnemosyne/internal/mcpserver"
)

// connect speaks to the real MCP server over an in-memory transport, exactly
// as an agent would over stdio. Going through the protocol rather than calling
// the store directly is the point: "tools" and "call" then exercise schema
// validation, annotations, and error mapping, so what they show is what the
// agent gets.
func connect(fs *flag.FlagSet, args []string) (*mcp.ClientSession, func(), error) {
	session, _, done, err := connectTo(fs, args)
	return session, done, err
}

// connectTo is connect plus a description of the root it opened, for a caller
// that has to tell the user where the call landed.
func connectTo(fs *flag.FlagSet, args []string) (*mcp.ClientSession, string, func(), error) {
	s, loc, root, _, err := openStore(fs, args)
	if err != nil {
		return nil, "", nil, err
	}
	where := root
	if loc.Dev {
		where += " (development build)"
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpserver.New(s).Connect(context.Background(), serverTransport, nil)
	if err != nil {
		s.Close()
		return nil, "", nil, err
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "mnemosyne-cli", Version: mcpserver.Version}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		serverSession.Close()
		s.Close()
		return nil, "", nil, err
	}

	return session, where, func() {
		session.Close()
		serverSession.Close()
		s.Close()
	}, nil
}

// toolsCmd prints what an agent sees on connect: the instructions the client
// injects into its context, then every tool with its arguments.
func toolsCmd(args []string) error {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
	verbose := fs.Bool("full", false, "print full descriptions and instructions")

	session, where, done, err := connectTo(fs, args)
	if err != nil {
		return err
	}
	defer done()

	if *verbose {
		fmt.Println("=== instructions sent to the agent ===")
		fmt.Println(session.InitializeResult().Instructions)
	}

	list, err := session.ListTools(context.Background(), nil)
	if err != nil {
		return err
	}

	fmt.Println("=== tools ===")
	for _, t := range list.Tools {
		kind := "write"
		if t.Annotations != nil && t.Annotations.ReadOnlyHint {
			kind = "read"
		}
		if t.Annotations != nil && t.Annotations.DestructiveHint != nil && *t.Annotations.DestructiveHint {
			kind = "destructive"
		}
		fmt.Printf("\n%-16s [%s]  %s\n", t.Name, kind, argsOf(t.InputSchema))
		fmt.Printf("  %s\n", wrap(t.Description, *verbose))
	}
	// Prompts and resources are part of what a client sees on connect, so a
	// command that claims to print "what an agent sees" has to show them too.
	if prompts, err := session.ListPrompts(context.Background(), nil); err == nil && len(prompts.Prompts) > 0 {
		fmt.Println("\n=== prompts ===")
		for _, p := range prompts.Prompts {
			names := make([]string, 0, len(p.Arguments))
			for _, a := range p.Arguments {
				if a.Required {
					names = append(names, a.Name)
				} else {
					names = append(names, a.Name+"?")
				}
			}
			fmt.Printf("\n%-16s (%s)\n", p.Name, strings.Join(names, ", "))
			fmt.Printf("  %s\n", wrap(p.Description, *verbose))
		}
	}

	if res, err := session.ListResources(context.Background(), nil); err == nil {
		fmt.Printf("\n=== resources ===\n\n%d memories advertised as mnemosyne://<project>/<slug>\n", len(res.Resources))
		for i, r := range res.Resources {
			if i == 5 {
				fmt.Printf("  … and %d more\n", len(res.Resources)-i)
				break
			}
			fmt.Printf("  %s\n", r.URI)
		}
	}

	cliPrefix := "mnemosyne"
	if strings.Contains(where, "development") {
		cliPrefix = "mnemosyne --dev"
	}
	fmt.Printf("\nCall one with:  %s call <tool> '{\"project\":\"...\"}'\n", cliPrefix)
	return nil
}

// callCmd invokes a single tool and prints its result as JSON, so a tool can
// be tried without wiring up an agent first.
func callCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mnemosyne call <tool> [json-args] — run \"mnemosyne tools\" to see them")
	}
	name, rest := args[0], args[1:]

	// Arguments come from the next positional word if it looks like JSON,
	// otherwise from stdin, so a big body can be piped in.
	payload := ""
	if len(rest) > 0 && strings.HasPrefix(strings.TrimSpace(rest[0]), "{") {
		payload, rest = rest[0], rest[1:]
	} else if stat, err := os.Stdin.Stat(); err == nil && stat.Mode()&os.ModeCharDevice == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		payload = strings.TrimSpace(string(data))
	}

	arguments := map[string]any{}
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &arguments); err != nil {
			return fmt.Errorf("arguments are not valid JSON: %w", err)
		}
	}

	session, where, done, err := connectTo(flag.NewFlagSet("call", flag.ContinueOnError), rest)
	if err != nil {
		return err
	}
	defer done()

	// Which root this landed in, on stderr so stdout stays the tool's JSON.
	//
	// Silence here is what makes the worst mistake invisible: an agent asked to
	// work in the development root runs the installed binary without
	// MNEMOSYNE_DEV, the write succeeds, and nothing in the result says it went
	// somewhere else. The user finds out by not seeing it in the app.
	fmt.Fprintf(os.Stderr, "root: %s\n", where)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return err
	}

	// Structured output and text content are the same payload for these tools,
	// so print the readable one and fall back to text only when there is none.
	if res.StructuredContent != nil {
		out, err := json.MarshalIndent(res.StructuredContent, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		for _, c := range res.Content {
			if text, ok := c.(*mcp.TextContent); ok {
				fmt.Println(text.Text)
			}
		}
	}
	if res.IsError {
		return fmt.Errorf("%s returned an error", name)
	}
	return nil
}

// argsOf renders a tool's input schema as a one-line signature.
func argsOf(schema any) string {
	data, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || len(parsed.Properties) == 0 {
		return "()"
	}

	required := map[string]bool{}
	for _, r := range parsed.Required {
		required[r] = true
	}
	var req, opt []string
	for name := range parsed.Properties {
		if required[name] {
			req = append(req, name)
		} else {
			opt = append(opt, name+"?")
		}
	}
	sort.Strings(req)
	sort.Strings(opt)
	return "(" + strings.Join(append(req, opt...), ", ") + ")"
}

// wrap keeps the tool list scannable: one line each unless --full is set.
func wrap(s string, full bool) string {
	s = strings.Join(strings.Fields(s), " ")
	if full || len(s) <= 100 {
		return s
	}
	return s[:97] + "..."
}
