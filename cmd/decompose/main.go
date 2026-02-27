package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/dusk-indust/decompose/internal/mcptools"
	"github.com/dusk-indust/decompose/internal/orchestrator"
)

// CLI flags parsed from command line.
type cliFlags struct {
	ProjectRoot string
	OutputDir   string
	Agents      string
	SingleAgent bool
	Verbose     bool
	ServeMCP    bool
	Version     bool
}

// version is set by goreleaser at build time.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var flags cliFlags

	fs := flag.NewFlagSet("decompose", flag.ContinueOnError)
	fs.StringVar(&flags.ProjectRoot, "project-root", ".", "path to the target project")
	fs.StringVar(&flags.OutputDir, "output-dir", "", "output directory for decomposition files")
	fs.StringVar(&flags.Agents, "agents", "", "comma-separated agent endpoint URLs")
	fs.BoolVar(&flags.SingleAgent, "single-agent", false, "force single-agent mode")
	fs.BoolVar(&flags.Verbose, "verbose", false, "enable verbose output")
	fs.BoolVar(&flags.ServeMCP, "serve-mcp", false, "run as MCP server for Claude Code integration")
	fs.BoolVar(&flags.Version, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if flags.Version {
		fmt.Println(version)
		return nil
	}

	// Build Config from flags (project root needed for both MCP and CLI modes).
	projectRoot := flags.ProjectRoot
	if !filepath.IsAbs(projectRoot) {
		abs, err := filepath.Abs(projectRoot)
		if err != nil {
			return fmt.Errorf("resolving project root: %w", err)
		}
		projectRoot = abs
	}

	// Create A2A HTTP client (used for both detection and pipeline).
	client := a2a.NewHTTPClient()
	ctx := context.Background()

	// --serve-mcp: start MCP server on stdio.
	if flags.ServeMCP {
		cfg := orchestrator.Config{
			ProjectRoot: projectRoot,
			Capability:  orchestrator.CapBasic,
			SingleAgent: flags.SingleAgent,
			Verbose:     flags.Verbose,
		}
		pipeline := orchestrator.NewPipeline(cfg, client)
		defer pipeline.Close()

		server := mcptools.NewDecomposeMCPServer(pipeline, cfg)
		return mcptools.RunDecomposeMCPServerStdio(ctx, server)
	}

	// Positional args: [name] [stage]
	positional := fs.Args()
	if len(positional) < 1 {
		return fmt.Errorf("usage: decompose [flags] <name> [stage]")
	}
	name := positional[0]

	outputDir := flags.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(projectRoot, "docs", "decompose", name)
	}

	// Determine capability level: use explicit --agents flag or auto-detect.
	cap := orchestrator.CapBasic
	var agentEndpoints []string
	if flags.Agents != "" {
		agentEndpoints = strings.Split(flags.Agents, ",")
		for i := range agentEndpoints {
			agentEndpoints[i] = strings.TrimSpace(agentEndpoints[i])
		}
		if len(agentEndpoints) > 0 {
			cap = orchestrator.CapA2AMCP
		}
	} else if !flags.SingleAgent {
		// Auto-detect capabilities.
		detector := orchestrator.NewDefaultDetector(client, flags.SingleAgent)
		detectedCap, detectedAgents, err := detector.Detect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: capability detection failed: %v\n", err)
		} else {
			cap = detectedCap
			agentEndpoints = detectedAgents
			fmt.Fprintf(os.Stderr, "Detected capability: %s\n", cap)
		}
	}
	if flags.SingleAgent {
		cap = orchestrator.CapBasic
	}

	cfg := orchestrator.Config{
		Name:           name,
		ProjectRoot:    projectRoot,
		OutputDir:      outputDir,
		Capability:     cap,
		AgentEndpoints: agentEndpoints,
		SingleAgent:    flags.SingleAgent,
		Verbose:        flags.Verbose,
	}

	// Create pipeline.
	pipeline := orchestrator.NewPipeline(cfg, client)

	// Drain progress events to stderr in a background goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range pipeline.Progress() {
			fmt.Fprintln(os.Stderr, orchestrator.FormatProgress(ev))
		}
	}()

	// Determine whether to run a single stage or the full pipeline.
	var runErr error
	if len(positional) >= 2 {
		stageNum, err := strconv.Atoi(positional[1])
		if err != nil {
			pipeline.Close()
			<-done
			return fmt.Errorf("invalid stage number %q: %w", positional[1], err)
		}
		if stageNum < 0 || stageNum > 4 {
			pipeline.Close()
			<-done
			return fmt.Errorf("stage must be 0-4, got %d", stageNum)
		}
		result, err := pipeline.RunStage(ctx, orchestrator.Stage(stageNum))
		if err != nil {
			runErr = err
		} else {
			for _, p := range result.FilePaths {
				fmt.Println(p)
			}
		}
	} else {
		results, err := pipeline.RunPipeline(ctx, orchestrator.StageDevelopmentStandards, orchestrator.StageTaskSpecifications)
		if err != nil {
			runErr = err
		} else {
			for _, r := range results {
				for _, p := range r.FilePaths {
					fmt.Println(p)
				}
			}
		}
	}

	// Close progress channel and wait for the drain goroutine.
	pipeline.Close()
	<-done

	return runErr
}
