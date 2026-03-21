package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/tui"
	flag "github.com/spf13/pflag"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Gateway host")
	port := flag.Int("port", 18791, "Gateway port")
	theme := flag.String("theme", "", "Initial theme (default from state)")
	session := flag.String("session", "", "Initial session key")
	flag.Parse()

	cfg := app.Config{
		Host:    *host,
		Port:    *port,
		Theme:   *theme,
		Session: *session,
	}

	model := tui.New(cfg)
	program := tea.NewProgram(model, tea.WithFilter(tui.FilterProgramMsg))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
