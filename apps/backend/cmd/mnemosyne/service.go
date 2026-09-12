package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aiyu-ayaan/mnemosyne/internal/channel"
	"github.com/aiyu-ayaan/mnemosyne/internal/config"
	"github.com/aiyu-ayaan/mnemosyne/internal/service"
)

const serviceUsage = `Usage: mnemosyne service <status|start|stop|install|uninstall>

The daemon is per-user and starts at logon on every platform. "mnemosyne
install" registers it; these subcommands inspect and override that.
`

// probeTimeout is short because the daemon is on a local pipe or socket: if it
// has not answered in this long, it is not going to.
const probeTimeout = 3 * time.Second

func stopCmd(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	loc, _, _, err := locate(fs, args)
	if err != nil {
		return err
	}

	stopped := stopChannelDaemon(loc)
	if err := service.Stop(); err == nil {
		stopped = true
	}

	if stopped {
		fmt.Println("daemon stopped")
	} else {
		fmt.Println("daemon is not running")
	}
	return nil
}

func stopChannelDaemon(loc config.Locations) bool {
	addr, err := channel.Resolve(loc.RuntimeDir, loc.Portable)
	if err != nil {
		return false
	}
	token, err := addr.ReadToken()
	if err != nil || token == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", channel.BaseURL+"/v1/shutdown", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := addr.HTTPClient().Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return true
}

func serviceCmd(args []string) error {
	if len(args) == 0 {
		fmt.Print(serviceUsage)
		return nil
	}

	switch args[0] {
	case "status":
		return serviceStatus(args[1:])

	case "start":
		if err := service.Start(); err != nil {
			return err
		}
		fmt.Println("daemon started")
		return nil

	case "stop":
		loc, _, _, _ := locate(flag.NewFlagSet("service stop", flag.ContinueOnError), args[1:])
		channelStopped := stopChannelDaemon(loc)
		svcErr := service.Stop()
		if !channelStopped && svcErr != nil && !errors.Is(svcErr, service.ErrNotRegistered) {
			return svcErr
		}
		fmt.Println("daemon stopped — it will start again at the next logon")
		return nil

	case "install":
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate the running binary: %w", err)
		}
		if err := service.Register(exe); err != nil {
			return err
		}
		fmt.Printf("registered %s to start at logon\n", exe)
		return nil

	case "uninstall":
		if err := service.Unregister(); err != nil {
			return err
		}
		fmt.Println("removed the logon entry — your memories were not touched")
		return nil

	case "help", "--help", "-h":
		fmt.Print(serviceUsage)
		return nil

	default:
		return fmt.Errorf("unknown service subcommand %q\n\n%s", args[0], serviceUsage)
	}
}

// serviceStatus reports both halves of "is it working": whether the logon entry
// exists, and whether anything actually answers on the channel.
//
// The second is the one that matters. Only it tells a registered-but-crashed
// daemon apart from a healthy one, and it exercises the channel, the token, and
// the store in one call.
func serviceStatus(args []string) error {
	loc, _, _, err := locate(flag.NewFlagSet("service status", flag.ContinueOnError), args)
	if err != nil {
		return err
	}

	state, err := service.Query()
	if err != nil {
		return err
	}

	fmt.Printf("mechanism     %s\n", state.Mechanism)
	fmt.Printf("logon entry   %s\n", label(state.Registered, "registered", "not registered"))
	if state.Path != "" {
		fmt.Printf("  at          %s\n", state.Path)
	}
	fmt.Printf("platform says %s\n", label(state.Running, "running", "not running"))

	addr, err := channel.Resolve(loc.RuntimeDir, loc.Portable)
	if err != nil {
		return err
	}
	fmt.Printf("channel       %s\n", addr.Endpoint)

	root, err := probeDaemon(addr)
	if err != nil {
		fmt.Printf("daemon        not reachable (%v)\n", err)
		return nil
	}
	fmt.Printf("daemon        answering, serving %s\n", root)
	return nil
}

// probeDaemon asks the daemon which root it is serving.
func probeDaemon(addr channel.Address) (string, error) {
	token, err := addr.ReadToken()
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("no token at %s", addr.TokenPath())
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", channel.BaseURL+"/v1/health", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := addr.HTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health returned %s", res.Status)
	}

	var health struct {
		Root string `json:"root"`
	}
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		return "", err
	}
	return health.Root, nil
}

func label(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}
