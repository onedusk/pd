package orchestrator

// CapabilityLevel describes the detected runtime capabilities.
// Determines which execution mode the orchestrator uses.
type CapabilityLevel int

const (
	// CapBasic is the fallback: no Go binary features, current /decompose behavior.
	CapBasic CapabilityLevel = iota

	// CapMCPOnly has MCP tools but no A2A agents. Single agent with enhanced tools.
	CapMCPOnly

	// CapA2AMCP has A2A agents and MCP tools but no code intelligence graph.
	CapA2AMCP

	// CapFull has A2A + MCP + code intelligence. Full parallel pipeline.
	CapFull
)

func (c CapabilityLevel) String() string {
	switch c {
	case CapBasic:
		return "basic"
	case CapMCPOnly:
		return "mcp-only"
	case CapA2AMCP:
		return "a2a+mcp"
	case CapFull:
		return "full"
	default:
		return "unknown"
	}
}

// Config holds runtime configuration for a decomposition run.
type Config struct {
	// Name is the decomposition name (kebab-case).
	Name string

	// ProjectRoot is the absolute path to the target project.
	ProjectRoot string

	// OutputDir is the path to docs/decompose/<name>/.
	OutputDir string

	// Stage0Path is the path to the shared development standards file.
	// Empty if Stage 0 does not exist.
	Stage0Path string

	// Capability is the detected runtime capability level.
	Capability CapabilityLevel

	// AgentEndpoints lists discovered specialist agent URLs.
	// Empty when Capability < CapA2AMCP.
	AgentEndpoints []string

	// InputFile is the path to a high-level input file that seeds Stage 1.
	InputFile string

	// InputContent is the inline content that seeds Stage 1 (alternative to InputFile).
	InputContent string

	// SingleAgent forces single-agent mode regardless of available capabilities.
	SingleAgent bool

	// Verbose enables agent-level progress output.
	Verbose bool
}
