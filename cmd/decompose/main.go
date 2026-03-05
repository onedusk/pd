package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/onedusk/pd/internal/agent"
	"github.com/onedusk/pd/internal/config"
	"github.com/onedusk/pd/internal/graph"
	"github.com/onedusk/pd/internal/mcptools"
	"github.com/onedusk/pd/internal/orchestrator"
	"github.com/onedusk/pd/internal/status"
)

// CLI flags parsed from command line.
type cliFlags struct {
	ProjectRoot      string
	OutputDir        string
	InputFile        string
	Agents           string
	SingleAgent      bool
	SkipVerification bool
	ReviewMode       string
	MaxConcurrent    int
	Verbose          bool
	ServeMCP         bool
	Force            bool
	SkipReview       bool
	Version          bool
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
	fs.StringVar(&flags.InputFile, "input", "", "path to a high-level input file (idea, spec, or plan) to seed Stage 1")
	fs.BoolVar(&flags.SkipVerification, "skip-verification", false, "skip post-stage verification")
	fs.StringVar(&flags.ReviewMode, "review-mode", "cli", "review strategy for implement command: cli, pr, file")
	fs.IntVar(&flags.MaxConcurrent, "max-concurrent", 3, "max parallel Claude Code sessions for implement command")
	fs.BoolVar(&flags.Force, "force", false, "overwrite existing files during init")
	fs.BoolVar(&flags.SkipReview, "skip-review", false, "suppress review warnings when implementing")
	fs.BoolVar(&flags.Version, "version", false, "print version and exit")

	fs.Usage = func() { printUsage(fs) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // --help is not an error
		}
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

	// Load project config (decompose.yml). CLI flags override config values.
	projCfg, err := config.Load(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load decompose.yml: %v\n", err)
		projCfg = &config.ProjectConfig{}
	}
	if projCfg.Verbose && !flags.Verbose {
		flags.Verbose = true
	}
	if projCfg.SingleAgent && !flags.SingleAgent {
		flags.SingleAgent = true
	}

	// Create A2A HTTP client (used for both detection and pipeline).
	client := a2a.NewHTTPClient()
	ctx := context.Background()

	// --serve-mcp: start unified MCP server on stdio with code intelligence.
	if flags.ServeMCP {
		cfg := orchestrator.Config{
			ProjectRoot: projectRoot,
			Capability:  orchestrator.CapMCPOnly,
			SingleAgent: flags.SingleAgent,
			Verbose:     flags.Verbose,
		}
		pipeline := orchestrator.NewPipeline(cfg, client)
		defer pipeline.Close()

		// Create code intelligence service with in-memory graph store + tree-sitter.
		store := graph.NewMemStore()
		parser := graph.NewTreeSitterParser()
		codeintel := mcptools.NewCodeIntelService(store, parser)
		codeintel.SetProjectRoot(projectRoot)

		fmt.Fprintf(os.Stderr, "decompose MCP server v%s starting on stdio (project: %s)\n", version, projectRoot)
		server := mcptools.NewUnifiedMCPServer(pipeline, cfg, codeintel)
		err := mcptools.RunUnifiedMCPServerStdio(ctx, server)
		fmt.Fprintf(os.Stderr, "decompose MCP server stopped\n")
		return err
	}

	// Handle subcommands.
	positional := fs.Args()
	if len(positional) > 0 && positional[0] == "init" {
		return runInit(projectRoot, flags.Force)
	}
	if len(positional) > 0 && positional[0] == "status" {
		name := ""
		if len(positional) > 1 {
			name = positional[1]
		}
		return runStatus(projectRoot, name)
	}
	if len(positional) > 0 && positional[0] == "export" {
		return runExport(projectRoot, positional[1:])
	}
	if len(positional) > 0 && positional[0] == "diagram" {
		return runDiagram(projectRoot)
	}
	if len(positional) > 0 && positional[0] == "augment" {
		pattern := ""
		if len(positional) > 1 {
			pattern = strings.Join(positional[1:], " ")
		}
		return runAugment(projectRoot, pattern)
	}
	if len(positional) > 0 && positional[0] == "review" {
		if len(positional) < 2 {
			return fmt.Errorf("usage: decompose review <name>")
		}
		return runReview(ctx, projectRoot, positional[1], flags)
	}
	if len(positional) > 0 && positional[0] == "review-interpret" {
		if len(positional) < 2 {
			return fmt.Errorf("usage: decompose review-interpret <name>")
		}
		return runReviewInterpret(ctx, projectRoot, positional[1], flags)
	}
	if len(positional) > 0 && positional[0] == "implement" {
		if len(positional) < 2 {
			return fmt.Errorf("usage: decompose implement <name>")
		}
		return runImplement(ctx, projectRoot, positional[1], flags)
	}

	// Positional args: [name] [stage]
	if len(positional) < 1 {
		printUsage(fs)
		return fmt.Errorf("missing command or decomposition name")
	}
	name := positional[0]

	outputDir := flags.OutputDir
	if outputDir == "" && projCfg.OutputDir != "" {
		outputDir = projCfg.OutputDir
	}
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
			fmt.Fprintf(os.Stderr, "  Using single-agent mode (basic template scaffolding).\n")
			fmt.Fprintf(os.Stderr, "  To use A2A agents, pass --agents <url1,url2,...>\n")
		} else {
			cap = detectedCap
			agentEndpoints = detectedAgents
			if flags.Verbose {
				fmt.Fprintf(os.Stderr, "Detected capability: %s\n", capDescription(cap))
			}
		}
	}
	if flags.SingleAgent {
		cap = orchestrator.CapBasic
	}

	cfg := orchestrator.Config{
		Name:             name,
		ProjectRoot:      projectRoot,
		OutputDir:        outputDir,
		InputFile:        flags.InputFile,
		Capability:       cap,
		AgentEndpoints:   agentEndpoints,
		SingleAgent:      flags.SingleAgent,
		SkipVerification: flags.SkipVerification,
		Verbose:          flags.Verbose,
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

func runImplement(ctx context.Context, projectRoot, name string, flags cliFlags) error {
	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)

	// Verify decomposition is complete.
	ds := status.GetDecompositionStatus(projectRoot, name)
	for _, s := range ds.Stages {
		if !s.Complete {
			return fmt.Errorf("stage %d (%s) is not complete; run full decomposition before implementing", s.Stage, s.Name)
		}
	}

	// Check review status (non-blocking warning).
	checkReviewBeforeImplement(projectRoot, name, flags.SkipReview)

	// Read and parse Stage 3 milestone dependencies.
	stage3Path := filepath.Join(outputDir, fmt.Sprintf("stage-3-%s.md", orchestrator.StageTaskIndex.String()))
	stage3Content, err := os.ReadFile(stage3Path)
	if err != nil {
		return fmt.Errorf("read stage 3: %w", err)
	}
	milestones, err := orchestrator.ParseMilestones(string(stage3Content))
	if err != nil {
		return fmt.Errorf("parse milestones: %w", err)
	}

	// Map milestones to task spec files.
	for i := range milestones {
		num := extractMilestoneNum(milestones[i].ID)
		milestones[i].TaskSpecPath = filepath.Join(outputDir, fmt.Sprintf("tasks_m%02d.md", num))
	}

	// Build scheduler.
	scheduler, err := orchestrator.NewScheduler(milestones)
	if err != nil {
		return fmt.Errorf("build scheduler: %w", err)
	}

	// Select review strategy.
	var review orchestrator.ReviewStrategy
	switch flags.ReviewMode {
	case "pr":
		review = orchestrator.NewPRReviewStrategy("main", "origin")
	case "file":
		review = orchestrator.NewFileReviewStrategy(filepath.Join(outputDir, "reviews"))
	default:
		review = orchestrator.NewCLIReviewStrategy(os.Stdin, os.Stdout)
	}

	// Create implementer function.
	impl := agent.NewImplementerAgent(projectRoot, outputDir)
	implementer := func(ctx context.Context, milestone *orchestrator.MilestoneNode) ([]orchestrator.ImplementationArtifact, error) {
		return impl.ImplementMilestone(ctx, milestone)
	}

	// Build and run implementation pipeline.
	cfg := orchestrator.ImplementConfig{
		Name:          name,
		ProjectRoot:   projectRoot,
		OutputDir:     outputDir,
		MaxConcurrent: flags.MaxConcurrent,
		Verbose:       flags.Verbose,
	}

	pipeline := orchestrator.NewImplementPipeline(cfg, scheduler, review, implementer)

	// Drain progress events in background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range pipeline.Progress() {
			fmt.Fprintln(os.Stderr, orchestrator.FormatProgress(ev))
		}
	}()

	runErr := pipeline.Run(ctx)

	pipeline.Close()
	<-done

	// Print summary.
	fmt.Println(orchestrator.FormatImplementationSummary(scheduler.Milestones()))

	return runErr
}

// extractMilestoneNum extracts the numeric part from a milestone ID like "M3".
func extractMilestoneNum(id string) int {
	n := 0
	for _, c := range id {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func capDescription(cap orchestrator.CapabilityLevel) string {
	switch cap {
	case orchestrator.CapBasic:
		return "basic (template scaffolding, no MCP or agents)"
	case orchestrator.CapMCPOnly:
		return "mcp-only (MCP tools available, single-agent mode)"
	case orchestrator.CapA2AMCP:
		return "a2a+mcp (A2A agents + MCP tools, parallel execution)"
	case orchestrator.CapFull:
		return "full (A2A + MCP + code intelligence)"
	default:
		return cap.String()
	}
}

func printUsage(fs *flag.FlagSet) {
	w := os.Stderr
	fmt.Fprintf(w, "decompose v%s — spec-driven development pipeline\n\n", version)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  decompose [flags] <name> [stage]    Run pipeline or single stage")
	fmt.Fprintln(w, "  decompose [flags] review <name>     Run review phase (codebase-plan cross-reference)")
	fmt.Fprintln(w, "  decompose [flags] review-interpret <name>  Interpretive triage of review findings")
	fmt.Fprintln(w, "  decompose [flags] implement <name>  Implement via Claude Code sessions")
	fmt.Fprintln(w, "  decompose [flags] init              Install skill, hooks, and MCP config")
	fmt.Fprintln(w, "  decompose [flags] status [name]     Show decomposition status")
	fmt.Fprintln(w, "  decompose [flags] export <name>     Export decomposition as JSON")
	fmt.Fprintln(w, "  decompose [flags] diagram           Generate Mermaid dependency diagram")
	fmt.Fprintln(w, "  decompose --serve-mcp               Run as MCP server on stdio")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Stages:")
	fmt.Fprintln(w, "  0  Development Standards    Team norms (shared, written once)")
	fmt.Fprintln(w, "  1  Design Pack              Research-grounded specification")
	fmt.Fprintln(w, "  2  Implementation Skeletons  Compilable type definitions")
	fmt.Fprintln(w, "  3  Task Index               Dependency-aware milestone plan")
	fmt.Fprintln(w, "  4  Task Specifications      Per-milestone executable tasks")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  decompose auth-system           Run full pipeline")
	fmt.Fprintln(w, "  decompose auth-system 1         Run Stage 1 only")
	fmt.Fprintln(w, "  decompose init                  Install into current project")
	fmt.Fprintln(w, "  decompose status                Show all decompositions")
	fmt.Fprintln(w, "  decompose --serve-mcp           Start MCP server")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fs.PrintDefaults()
}
