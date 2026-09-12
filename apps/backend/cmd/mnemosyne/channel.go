package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aiyu-ayaan/mnemosyne/internal/channel"
)

// channelInfo is everything a local client needs to talk to the daemon.
//
// It exists so that the desktop app does not reimplement root resolution,
// portable detection, and the per-platform endpoint naming in TypeScript. One
// subprocess call and the client has the answer the binary itself would give,
// which means the two can never disagree about where the daemon is.
type channelInfo struct {
	Endpoint  string `json:"endpoint"`
	BaseURL   string `json:"baseUrl"`
	TokenPath string `json:"tokenPath"`
	Token     string `json:"token,omitempty"`
	Root      string `json:"root"`
	Portable  bool   `json:"portable"`
	Running   bool   `json:"running"`
}

// channelCmd prints the channel details. The token is included: any process
// running as this user can already read the token file, so withholding it here
// would protect nothing and only force every client to reimplement the read.
func channelCmd(args []string) error {
	fs := flag.NewFlagSet("channel", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print as JSON")

	loc, root, _, err := locate(fs, args)
	if err != nil {
		return err
	}

	addr, err := channel.Resolve(loc.RuntimeDir, loc.Portable)
	if err != nil {
		return err
	}
	token, err := addr.ReadToken()
	if err != nil {
		return err
	}

	info := channelInfo{
		Endpoint:  addr.Endpoint,
		BaseURL:   channel.BaseURL,
		TokenPath: addr.TokenPath(),
		Token:     token,
		Root:      root,
		Portable:  loc.Portable,
	}
	if _, err := probeDaemon(addr); err == nil {
		info.Running = true
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	fmt.Printf("endpoint    %s\n", info.Endpoint)
	fmt.Printf("token file  %s\n", info.TokenPath)
	fmt.Printf("memory root %s\n", info.Root)
	fmt.Printf("daemon      %s\n", label(info.Running, "answering", "not reachable"))
	return nil
}
