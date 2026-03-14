package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.zoe.im/agentbox/pkg/agent"
	"go.zoe.im/x/cli"
	"sigs.k8s.io/yaml"
)

var agentExtractCmd = cli.New(
	cli.Name("extract"),
	cli.Short("Extract experience packs from a MEMORY.md file"),
	cli.Run(runAgentExtract),
)

var agentRefineCmd = cli.New(
	cli.Name("refine"),
	cli.Short("Apply L2 AI refinement to experience packs (requires LLM)"),
	cli.Run(runAgentRefine),
)

var (
	extractOutput  string
	extractFormat  string
	refineBaseURL  string
	refineAPIKey   string
	refineModel    string
)

func init() {
	agentExtractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "Output directory for experience packs (default: stdout)")
	agentExtractCmd.Flags().StringVarP(&extractFormat, "format", "f", "yaml", "Output format (yaml|json|markdown)")

	agentRefineCmd.Flags().StringVar(&refineBaseURL, "base-url", "", "LLM API base URL (or AGENTBOX_LLM_URL env)")
	agentRefineCmd.Flags().StringVar(&refineAPIKey, "api-key", "", "LLM API key (or AGENTBOX_LLM_KEY env)")
	agentRefineCmd.Flags().StringVar(&refineModel, "model", "claude-sonnet-4-20250514", "LLM model to use")
}

func runAgentExtract(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent extract <MEMORY.md> [--output dir] [--format yaml|json|markdown]")
		os.Exit(1)
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", args[0], err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Extracting experiences from %s...\n", args[0])
	packs := agent.MemoryToExperiences(string(data))

	if len(packs) == 0 {
		fmt.Fprintln(os.Stderr, "No experiences found.")
		return
	}

	fmt.Fprintf(os.Stderr, "Found %d experience packs:\n", len(packs))
	for i, p := range packs {
		fmt.Fprintf(os.Stderr, "  %d. [%s] %s (%s)\n", i+1, p.ID, p.Summary, p.Domain)
		if len(p.Tags) > 0 {
			fmt.Fprintf(os.Stderr, "     tags: %v\n", p.Tags)
		}
	}

	if extractOutput != "" {
		// Write to directory
		if err := os.MkdirAll(extractOutput, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating dir: %v\n", err)
			os.Exit(1)
		}

		for _, p := range packs {
			var data []byte
			var ext string

			switch extractFormat {
			case "json":
				data, _ = json.MarshalIndent(p, "", "  ")
				ext = ".json"
			case "markdown":
				data = experienceToMarkdown(&p)
				ext = ".md"
			default:
				data, _ = yaml.Marshal(p)
				ext = ".yaml"
			}

			path := filepath.Join(extractOutput, p.ID+ext)
			if err := os.WriteFile(path, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "  → %s\n", path)
		}

		// Write index
		idx := agent.ExperienceIndex{
			Packs: packs,
		}
		idxData, _ := yaml.Marshal(idx)
		idxPath := filepath.Join(extractOutput, "index.yaml")
		os.WriteFile(idxPath, idxData, 0644)
		fmt.Fprintf(os.Stderr, "  → %s\n", idxPath)
	} else {
		// Output to stdout
		switch extractFormat {
		case "json":
			data, _ := json.MarshalIndent(packs, "", "  ")
			fmt.Println(string(data))
		default:
			data, _ := yaml.Marshal(packs)
			fmt.Println(string(data))
		}
	}
}

func experienceToMarkdown(p *agent.ExperiencePack) []byte {
	var b []byte
	b = append(b, fmt.Sprintf("# %s\n\n", p.Summary)...)
	b = append(b, fmt.Sprintf("## 领域\n%s\n\n", p.Domain)...)
	if p.Difficulty != "" {
		b = append(b, fmt.Sprintf("## 难度\n%s\n\n", p.Difficulty)...)
	}
	b = append(b, fmt.Sprintf("## 摘要\n%s\n\n", p.Summary)...)
	b = append(b, fmt.Sprintf("## 详情\n%s\n", p.Detail)...)
	if len(p.Tags) > 0 {
		b = append(b, "\n## 标签\n"...)
		for _, t := range p.Tags {
			b = append(b, fmt.Sprintf("- %s\n", t)...)
		}
	}
	return b
}

// --- sanitize command ---

var agentSanitizeCmd = cli.New(
	cli.Name("sanitize"),
	cli.Short("Apply L1 sanitization to experience packs"),
	cli.Run(runAgentSanitize),
)

func runAgentSanitize(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent sanitize <experience-dir-or-file>")
		os.Exit(1)
	}

	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		// Sanitize all packs in directory
		idx, err := agent.ParseExperienceDir(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		total := 0
		for i := range idx.Packs {
			applied := agent.SanitizeExperiencePack(&idx.Packs[i])
			if len(applied) > 0 {
				fmt.Printf("  %s: sanitized (%v)\n", idx.Packs[i].ID, applied)
				total += len(applied)
			} else {
				fmt.Printf("  %s: clean\n", idx.Packs[i].ID)
			}
		}

		fmt.Printf("\nSanitized %d packs, %d redactions applied.\n", len(idx.Packs), total)
	} else {
		// Single file
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		result, applied := agent.SanitizeL1Text(string(data))
		if len(applied) > 0 {
			fmt.Fprintf(os.Stderr, "Applied %d redaction types: %v\n", len(applied), applied)
		} else {
			fmt.Fprintln(os.Stderr, "No PII detected.")
		}
		fmt.Print(result)
	}
}

// --- refine command ---

func runAgentRefine(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent refine <experience-dir> [--base-url URL] [--api-key KEY]")
		os.Exit(1)
	}

	// Resolve config
	baseURL := refineBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("AGENTBOX_LLM_URL")
	}
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --base-url or AGENTBOX_LLM_URL is required")
		os.Exit(1)
	}

	apiKey := refineAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("AGENTBOX_LLM_KEY")
	}

	cfg := agent.L2RefineConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   refineModel,
	}

	dir := args[0]
	idx, err := agent.ParseExperienceDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(idx.Packs) == 0 {
		fmt.Fprintln(os.Stderr, "No experience packs found.")
		return
	}

	fmt.Fprintf(os.Stderr, "Refining %d experience packs with %s...\n", len(idx.Packs), cfg.Model)

	for i := range idx.Packs {
		p := &idx.Packs[i]
		fmt.Fprintf(os.Stderr, "  [%d/%d] %s ... ", i+1, len(idx.Packs), p.ID)

		if err := agent.RefineL2(p, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "✅\n")

		// Write back refined pack
		data, _ := yaml.Marshal(p)
		outPath := filepath.Join(dir, p.ID+".yaml")
		os.WriteFile(outPath, data, 0644)
	}

	// Update index
	idxData, _ := yaml.Marshal(idx)
	os.WriteFile(filepath.Join(dir, "index.yaml"), idxData, 0644)

	fmt.Fprintf(os.Stderr, "\nDone. Refined packs written to %s\n", dir)
}
