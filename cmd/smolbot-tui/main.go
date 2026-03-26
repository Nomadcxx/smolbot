package main

import (
	"fmt"
	"os"
	"sort"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/tui"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
	flag "github.com/spf13/pflag"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Gateway host")
	port := flag.Int("port", 18791, "Gateway port")
	theme := flag.String("theme", "", "Initial theme (default from state)")
	session := flag.String("session", "", "Initial session key")
	flag.Parse()

	cfg := app.Config{
		Host:       *host,
		Port:       *port,
		Theme:      *theme,
		Session:    *session,
		MCPServers: loadSidebarMCPServers(),
	}

	model := tui.New(cfg)
	program := tea.NewProgram(model, tea.WithFilter(tui.FilterProgramMsg))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadSidebarMCPServers() []client.MCPServerInfo {
	loaded, err := cfgpkg.Load(cfgpkg.DefaultPaths().ConfigFile())
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(loaded.Tools.MCPServers))
	for name := range loaded.Tools.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]client.MCPServerInfo, 0, len(names))
	for _, name := range names {
		server := loaded.Tools.MCPServers[name]
		tools := 0
		for _, enabled := range server.EnabledTools {
			if enabled == "*" {
				tools = 0
				break
			}
			tools++
		}
		servers = append(servers, client.MCPServerInfo{
			Name:   name,
			Status: "configured",
			Tools:  tools,
		})
	}
	return servers
}
