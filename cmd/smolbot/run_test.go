package main

import (
	"testing"
)

func TestRunCmdDoesNotShadowRootFlags(t *testing.T) {
	rootCmd := NewRootCmd("test")
	runCmd := newRunCmd(&rootOptions{configPath: "/default/path"})

	rootCmd.AddCommand(runCmd)

	protected := []string{"config", "workspace", "verbose"}
	for _, name := range protected {
		flag := runCmd.Flags().Lookup(name)
		if flag != nil {
			t.Errorf("run subcommand defines --%s flag that should be inherited from root", name)
		}
	}
}

func TestRunCmdDefinesPortFlag(t *testing.T) {
	runCmd := newRunCmd(&rootOptions{configPath: "/default/path"})
	flag := runCmd.Flags().Lookup("port")
	if flag == nil {
		t.Fatal("run subcommand should define --port flag")
	}
	if flag.DefValue != "18790" {
		t.Fatalf("port default = %q, want 18790", flag.DefValue)
	}
}