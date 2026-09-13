package main

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/aiyu-ayaan/mnemosyne/internal/config"
)

func TestLocateDevFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	loc, _, _, err := locate(fs, []string{"--dev"})
	if err != nil {
		t.Fatalf("locate --dev: %v", err)
	}
	if !loc.Dev {
		t.Errorf("locate with --dev: got loc.Dev = false, want true")
	}
}

func TestRunDevFlag(t *testing.T) {
	t.Setenv(config.EnvDev, "")
	err := run([]string{"--dev", "unknown-command"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
	if os.Getenv(config.EnvDev) != "1" {
		t.Errorf("run with --dev did not set MNEMOSYNE_DEV=1 in environment")
	}
}

func TestToolNamedDevSuggestion(t *testing.T) {
	t.Setenv(config.EnvDev, "1")
	err := run([]string{"list_projects"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mnemosyne --dev call list_projects '{}'") {
		t.Errorf("expected suggestion with --dev, got %v", err)
	}
}
