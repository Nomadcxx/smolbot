package main

import (
	"fmt"
	"os"

	"github.com/Nomadcxx/nanobot-go/pkg/agent"
	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/skill"
	"github.com/Nomadcxx/nanobot-go/pkg/tokenizer"
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

	tok := tokenizer.New()
	tokenCount := tok.EstimateTokens(prompt)

	// Count skills section specifically
	summary := reg.SummaryXML()
	summaryTokens := tok.EstimateTokens(summary)

	// Calculate what full content would be
	fullContentTokens := 0
	for _, name := range reg.Names() {
		s, _ := reg.Get(name)
		if s != nil {
			fullContentTokens += tok.EstimateTokens(s.Content)
		}
	}

	// Calculate savings
	savings := float64(fullContentTokens-summaryTokens) / float64(fullContentTokens) * 100

	fmt.Println("=== Token Usage Analysis ===")
	fmt.Printf("\nTotal prompt size: %d characters\n", len(prompt))
	fmt.Printf("Total prompt tokens: %d\n", tokenCount)

	fmt.Printf("\n--- Skills Section ---\n")
	fmt.Printf("Skills metadata (current): %d characters (%d tokens)\n", len(summary), summaryTokens)
	fmt.Printf("Skills full content (if loaded): %d tokens\n", fullContentTokens)
	fmt.Printf("Token savings from progressive disclosure: %.1f%%\n", savings)

	fmt.Printf("\n--- Breakdown ---\n")
	totalSkills := len(reg.Names())
	fmt.Printf("Total skills: %d\n", totalSkills)
	if totalSkills > 0 {
		fmt.Printf("Avg metadata per skill: %.1f tokens\n", float64(summaryTokens)/float64(totalSkills))
		fmt.Printf("Avg full content per skill: %.1f tokens\n", float64(fullContentTokens)/float64(totalSkills))
	}

	fmt.Println("\n=== Comparison ===")
	fmt.Println("Without progressive disclosure (full content always loaded):")
	fmt.Printf("  Skills tokens: %d\n", fullContentTokens)
	fmt.Println("With progressive disclosure (metadata only, content on demand):")
	fmt.Printf("  Skills tokens: %d\n", summaryTokens)
	fmt.Printf("  Savings: %.1f%% (%d tokens)\n", savings, fullContentTokens-summaryTokens)
}
