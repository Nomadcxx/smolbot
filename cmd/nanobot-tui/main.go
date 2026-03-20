package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui"
	flag "github.com/spf13/pflag"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	model := tui.New(cfg)
	program := tea.NewProgram(model, tea.WithFilter(tui.FilterProgramMsg))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (tui.Config, error) {
	fs := flag.NewFlagSet("nanobot-tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	host := fs.String("host", "127.0.0.1", "Gateway host")
	port := fs.Int("port", 18790, "Gateway port")
	theme := fs.String("theme", "", "Initial theme (default from state)")
	session := fs.String("session", "", "Initial session key")
	if err := fs.Parse(args); err != nil {
		return tui.Config{}, err
	}

	return tui.Config{
		Host:    *host,
		Port:    *port,
		Theme:   *theme,
		Session: *session,
	}, nil
}
