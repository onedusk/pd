#!/bin/bash
# Hook: augments built-in file tools with graph context from the decompose
# code intelligence database. Follows the GitNexus augmentation pattern —
# tools are never blocked, only enriched with additional context.
#
# All stderr is silenced to prevent Claude Code from reporting "hook error"
# on benign warnings from jq, timeout, or KuzuDB.

exec 2>/dev/null

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name')

# Only augment file operation tools
if [[ ! "$TOOL_NAME" =~ ^(Read|Write|Edit|Glob|Grep|Bash)$ ]]; then
  exit 0
fi

# Find the decompose binary from .mcp.json
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DECOMPOSE_BIN=$(jq -r '.mcpServers.decompose.command // empty' "$PROJECT_ROOT/.mcp.json" 2>/dev/null)

if [[ -z "$DECOMPOSE_BIN" ]] || [[ ! -x "$DECOMPOSE_BIN" ]]; then
  exit 0  # No binary available, allow tool without augmentation
fi

# Check if graph index exists (graceful degradation like GitNexus)
if [[ ! -e "$PROJECT_ROOT/.decompose/graph" ]]; then
  exit 0  # No index built yet, allow tool without augmentation
fi

# Extract search pattern from tool input
TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty')
PATTERN=""

case "$TOOL_NAME" in
  Grep)
    PATTERN=$(echo "$TOOL_INPUT" | jq -r '.pattern // empty')
    ;;
  Glob)
    # Extract meaningful keywords from glob pattern (3+ alpha chars)
    PATTERN=$(echo "$TOOL_INPUT" | jq -r '.pattern // empty' | grep -oE '[a-zA-Z]{3,}' | head -3 | tr '\n' ' ')
    ;;
  Read)
    # For Read, extract the filename stem as pattern
    PATTERN=$(echo "$TOOL_INPUT" | jq -r '.file_path // empty' | xargs basename 2>/dev/null | sed 's/\.[^.]*$//')
    ;;
  Bash)
    # Extract pattern from grep/rg commands in the bash command
    CMD=$(echo "$TOOL_INPUT" | jq -r '.command // empty')
    PATTERN=$(echo "$CMD" | grep -oE '(grep|rg)\s+[^|]+' | sed 's/^[^ ]* //' | sed 's/ .*//' | tr -d "\"'" | head -1)
    ;;
  Write|Edit)
    # For writes to docs/decompose, gently suggest write_stage
    FILE_PATH=$(echo "$TOOL_INPUT" | jq -r '.file_path // empty')
    if echo "$FILE_PATH" | grep -q "docs/decompose"; then
      cat << 'WRITEEOF'
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "additionalContext": "TIP: The mcp__decompose__write_stage tool writes decomposition files with automatic section validation, coherence checking, and correct ordering. Consider using it for stage output files."
  }
}
WRITEEOF
      exit 0
    fi
    exit 0
    ;;
esac

if [[ -z "$PATTERN" ]] || [[ ${#PATTERN} -lt 3 ]]; then
  exit 0  # Pattern too short to be useful
fi

# Query the graph (5s timeout)
GRAPH_CONTEXT=$(timeout 5 "$DECOMPOSE_BIN" --project-root "$PROJECT_ROOT" augment "$PATTERN")

if [[ -z "$GRAPH_CONTEXT" ]]; then
  exit 0  # No graph results, allow tool without augmentation
fi

# Return augmented context (same format as GitNexus)
jq -n --arg ctx "$GRAPH_CONTEXT" '{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "additionalContext": $ctx
  }
}'
exit 0
