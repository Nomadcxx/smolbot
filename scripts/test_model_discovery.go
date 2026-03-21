package main

import (
	"fmt"
	"os"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func main() {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Providers["ollama"] = config.ProviderConfig{
		APIBase: "http://127.0.0.1:11434",
	}

	models, err := provider.GetAvailableModels(&cfg)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d models:\n", len(models))
	for _, m := range models {
		fmt.Printf("  - %s (provider: %s)\n", m.Name, m.Provider)
	}
}
