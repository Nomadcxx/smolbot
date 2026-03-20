package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui"
	"github.com/spf13/pflag"
)

func main() {
	help, cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if help {
		fmt.Println("Usage of nanobot-tui:")
		fmt.Println("  --host string      Gateway host (default \"127.0.0.1\")")
		fmt.Println("  --port int         Gateway port (default 18790)")
		fmt.Println("  --session string   Initial session key")
		fmt.Println("  --theme string     Initial theme (default from state)")
		return
	}

	model := tui.New(cfg)
	program := tea.NewProgram(model, tea.WithFilter(tui.FilterProgramMsg))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (bool, tui.Config, error) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true, tui.Config{}, nil
		}
	}
	fs := pflag.NewFlagSet("nanobot-tui", pflag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "Gateway host")
	port := fs.Int("port", 18790, "Gateway port")
	theme := fs.String("theme", "", "Initial theme (default from state)")
	session := fs.String("session", "", "Initial session key")
	if err := fs.Parse(args); err != nil {
		return false, tui.Config{}, err
	}

	return false, tui.Config{
		Host:    *host,
		Port:    *port,
		Theme:   *theme,
		Session: *session,
	}, nil
}
