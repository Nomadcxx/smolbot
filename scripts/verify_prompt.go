package main

import (
	"fmt"
	"os"

	"github.com/Nomadcxx/nanobot-go/pkg/agent"
	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/skill"
)

func main() {
	workspace := os.Getenv("HOME") + "/.nanobot-go/workspace"
	paths := config.DefaultPaths()

	reg, err := skill.NewRegistry(paths)
	if err != nil {
		fmt.Printf("Error creating registry: %v\n", err)
		os.Exit(1)
	}

	ctx := agent.BuildContext{
		Workspace: workspace,
		Skills:    reg,
	}

	prompt, err := agent.BuildSystemPrompt(ctx)
	if err != nil {
		fmt.Printf("Error building prompt: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== System Prompt ===")
	fmt.Println(prompt)
	fmt.Println("\n=== Available Skills ===")
	for _, name := range reg.Names() {
		s, _ := reg.Get(name)
		fmt.Printf("- %s: %s (always=%v)\n", name, s.Description, s.Always)
	}
}
