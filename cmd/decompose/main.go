package main

import (
	"flag"
	"fmt"
	"os"
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

	// Orchestrator wiring will be implemented in M6.
	_ = flags
	return nil
}
