package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandWiring(t *testing.T) {
	cmd := NewRootCmd("test")

	want := []string{"run", "chat", "status", "onboard", "channels"}
	for _, name := range want {
		if _, _, err := cmd.Find([]string{name}); err != nil {
			t.Fatalf("expected subcommand %q: %v", name, err)
		}
	}
	if _, _, err := cmd.Find([]string{"channels", "status"}); err != nil {
		t.Fatalf("expected channels status subcommand: %v", err)
	}
	if _, _, err := cmd.Find([]string{"channels", "login"}); err != nil {
		t.Fatalf("expected channels login subcommand: %v", err)
	}
}

func TestRunFlagsAndHelp(t *testing.T) {
	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	help := out.String()
	for _, token := range []string{"run", "chat", "status", "onboard", "channels"} {
		if !strings.Contains(help, token) {
			t.Fatalf("expected help to mention %q, got %q", token, help)
		}
	}

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find run: %v", err)
	}
	if runCmd.Flags().Lookup("port") == nil {
		t.Fatal("expected run flag port")
	}
	for _, flag := range []string{"workspace", "config", "verbose"} {
		if runCmd.InheritedFlags().Lookup(flag) == nil {
			t.Fatalf("expected inherited flag %q", flag)
		}
		if runCmd.LocalFlags().Lookup(flag) != nil {
			t.Errorf("run command should not locally define inherited flag %q", flag)
		}
	}
}
