// Command mnemosyne is the Mnemosyne memory server.
//
// It speaks MCP over stdio, so stdout carries JSON-RPC and nothing else. All
// logging goes to stderr.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/daemon"
	"github.com/aiyu-ayaan/mnemosyne/internal/install"
	"github.com/aiyu-ayaan/mnemosyne/internal/mcpserver"
	"github.com/aiyu-ayaan/mnemosyne/internal/service"
	"github.com/aiyu-ayaan/mnemosyne/internal/store"
)

const usage = `Mnemosyne — local memory server for AI agents.

Usage:
  mnemosyne <command> [flags]

Commands:
  serve       Run the MCP server over stdio (this is what an agent starts)
  daemon      Run the background service the desktop app connects to
  doctor      Show the resolved setup and what is in it
  root        Print or set the memory root
  install     Install the binary, put it on PATH, and start it at logon
  uninstall   Undo an install
  service     Manage the logon entry: status, start, stop, install, uninstall
  channel     Print where the daemon listens (--json for a client to read)
  version     Print the version

Flags:
  --root <path>   Use this memory root for the current command
  --portable      Keep everything beside the binary and touch nothing else

The memory root is resolved in this order: --root, then $MNEMOSYNE_ROOT, then
the config file, then the default location. Run "mnemosyne doctor" to see which
one is in effect.

Portable mode also turns on automatically when a file named mnemosyne.portable
sits next to the binary.
`

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mnemosyne:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	command, rest := args[0], args[1:]
	switch command {
	case "serve":
		return serve(rest)
	case "daemon":
		return daemonCmd(rest)
	case "doctor":
		return doctor(rest)
	case "root":
		return rootCmd(rest)
	case "install":
		return installCmd(rest)
	case "uninstall":
		return uninstallCmd(rest)
	case "service":
		return serviceCmd(rest)
	case "channel":
		return channelCmd(rest)
	case "version", "--version", "-v":
		fmt.Println("mnemosyne", mcpserver.Version)
		return nil
	case "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q — run \"mnemosyne help\"", command)
	}
}

// locate parses the shared flags and resolves the layout and the memory root.
func locate(fs *flag.FlagSet, args []string) (config.Locations, string, config.Source, error) {
	root := fs.String("root", "", "memory root directory")
	portable := fs.Bool("portable", false, "keep everything beside the binary")
	if err := fs.Parse(args); err != nil {
		return config.Locations{}, "", "", err
	}

	loc, err := config.Detect(*portable)
	if err != nil {
		return config.Locations{}, "", "", err
	}
	resolved, source, err := loc.ResolveRoot(*root)
	if err != nil {
		return config.Locations{}, "", "", err
	}
	return loc, resolved, source, nil
}

// openStore resolves the layout and opens the store.
func openStore(fs *flag.FlagSet, args []string) (*store.Store, config.Locations, string, config.Source, error) {
	loc, resolved, source, err := locate(fs, args)
	if err != nil {
		return nil, loc, "", "", err
	}
	s, err := store.Open(resolved)
	if err != nil {
		return nil, loc, "", "", err
	}
	return s, loc, resolved, source, nil
}

func serve(args []string) error {
	s, _, root, _, err := openStore(flag.NewFlagSet("serve", flag.ContinueOnError), args)
	if err != nil {
		return err
	}
	defer s.Close()

	// Ctrl-C and a client shutdown both need to close the index cleanly, or
	// Windows leaves the database file locked.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("mnemosyne serving over stdio", "root", root, "version", mcpserver.Version)
	return mcpserver.Serve(ctx, s)
}

// daemonCmd runs the long-lived process the desktop app talks to. It takes
// ownership of the store, so there is no Close here.
func daemonCmd(args []string) error {
	s, loc, root, _, err := openStore(flag.NewFlagSet("daemon", flag.ContinueOnError), args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("mnemosyne daemon starting", "root", root, "version", mcpserver.Version)
	return daemon.Run(ctx, daemon.Options{Store: s, Locations: loc})
}

func doctor(args []string) error {
	s, loc, root, source, err := openStore(flag.NewFlagSet("doctor", flag.ContinueOnError), args)
	if err != nil {
		return err
	}
	defer s.Close()

	projects, err := s.ListProjects()
	if err != nil {
		return err
	}
	memories := 0
	for _, p := range projects {
		memories += p.MemoryCount
	}

	indexState := "missing (it will be rebuilt on next start)"
	if info, err := os.Stat(s.InternalPath(store.IndexFile)); err == nil {
		indexState = fmt.Sprintf("ok, %d KB", info.Size()/1024)
	}

	mode := "installed"
	if loc.Portable {
		mode = "portable"
	}

	fmt.Printf("version       %s\n", mcpserver.Version)
	fmt.Printf("mode          %s\n", mode)
	fmt.Printf("binary dir    %s\n", loc.BinDir)
	fmt.Printf("memory root   %s\n", root)
	fmt.Printf("  chosen by   %s\n", source)
	fmt.Printf("config file   %s\n", loc.ConfigPath)
	fmt.Printf("index         %s\n", indexState)
	fmt.Printf("projects      %d\n", len(projects))
	fmt.Printf("memories      %d\n", memories)

	for _, p := range projects {
		fmt.Printf("  %-30s %d\n", p.Slug, p.MemoryCount)
	}
	return nil
}

// rootCmd prints the memory root, or sets it when given a path.
func rootCmd(args []string) error {
	fs := flag.NewFlagSet("root", flag.ContinueOnError)
	loc, resolved, source, err := locate(fs, args)
	if err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fmt.Printf("%s (%s)\n", resolved, source)
		return nil
	}

	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("resolve %q: %w", fs.Arg(0), err)
	}

	cfg, err := loc.Load()
	if err != nil {
		return err
	}
	cfg.Root = abs
	if err := loc.Save(cfg); err != nil {
		return err
	}

	// Opening it here creates the directory and builds the index, so the next
	// agent to connect does not pay for a cold start.
	s, err := store.Open(abs)
	if err != nil {
		return err
	}
	defer s.Close()

	fmt.Printf("memory root set to %s\n", abs)
	return nil
}

func installCmd(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	machine := fs.Bool("machine", false, "install for every account on this machine (needs admin)")
	noPath := fs.Bool("no-path", false, "do not modify PATH")
	noAutostart := fs.Bool("no-autostart", false, "do not start the daemon at logon")
	if err := fs.Parse(args); err != nil {
		return err
	}

	loc, err := config.Detect(false)
	if err != nil {
		return err
	}
	// Installing is the opposite of what portable means. Doing it anyway would
	// leave traces on a machine the user meant to leave clean.
	if loc.Portable {
		return fmt.Errorf("this is a portable copy (%s is present) — remove that file first if you want to install",
			config.PortableMarker)
	}

	scope := install.User
	if *machine {
		scope = install.Machine
	}

	res, err := install.Run(install.Options{Scope: scope, SkipPath: *noPath})
	if err != nil {
		return err
	}

	fmt.Printf("installed %s (%s scope)\n", res.BinaryPath, scope)
	switch {
	case *noPath:
		fmt.Printf("PATH unchanged, as asked\n")
	case res.PathNote != "":
		fmt.Println(res.PathNote)
	case res.PathAdded:
		fmt.Printf("added %s to your PATH — open a new terminal to pick it up\n", res.Dir)
	default:
		fmt.Printf("%s was already on your PATH\n", res.Dir)
	}

	// The logon entry points at the installed copy, not the binary being run,
	// which may be a build sitting in a temp directory.
	switch {
	case *noAutostart:
		fmt.Println("logon autostart skipped, as asked")
	default:
		if err := service.Register(res.BinaryPath); err != nil {
			// A failed registration does not undo a good install: the binary is
			// on PATH and the MCP server works with no daemon at all.
			fmt.Printf("could not register the logon autostart: %v\n", err)
			fmt.Println(`run "mnemosyne service install" to retry`)
		} else {
			fmt.Println(`the daemon starts at logon — "mnemosyne service start" starts it now`)
		}
	}

	fmt.Printf("\nWire it into an agent with:\n\n    claude mcp add mnemosyne -- mnemosyne serve\n")
	return nil
}

func uninstallCmd(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	machine := fs.Bool("machine", false, "remove the machine-scope install (needs admin)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	scope := install.User
	if *machine {
		scope = install.Machine
	}

	// The logon entry goes first: leaving one pointing at a deleted binary
	// would make the next logon fail instead of doing nothing.
	if err := service.Unregister(); err != nil {
		fmt.Printf("could not remove the logon entry: %v\n", err)
	}

	res, err := install.Remove(scope)
	if err != nil {
		return err
	}

	fmt.Printf("removed %s and its PATH entry\n", res.BinaryPath)
	fmt.Printf("your memories were not touched — delete the memory root yourself if you want them gone\n")
	return nil
}
