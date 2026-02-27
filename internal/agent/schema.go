package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Compile-time interface check.
var _ Agent = (*SchemaAgent)(nil)

// SchemaAgent translates schemas, validates types, and writes API contracts.
// It embeds BaseAgent and routes incoming messages to the appropriate skill
// handler based on message content.
type SchemaAgent struct {
	*BaseAgent
}

// NewSchemaAgent creates a SchemaAgent with its agent card and skill set.
func NewSchemaAgent() *SchemaAgent {
	card := a2a.AgentCard{
		Name:        "schema-agent",
		Description: "Translates schemas, validates types, and writes API contracts",
		Version:     "dev",
		Skills: []a2a.AgentSkill{
			{
				ID:          "translate-schema",
				Name:        "Translate Schema",
				Description: "Parses entity descriptions and generates Go struct definitions",
				Tags:        []string{"schema", "codegen", "go"},
			},
			{
				ID:          "validate-types",
				Name:        "Validate Types",
				Description: "Validates type definitions and checks for correctness",
				Tags:        []string{"schema", "validation"},
			},
			{
				ID:          "write-contracts",
				Name:        "Write Contracts",
				Description: "Generates request/response struct pairs for API endpoints",
				Tags:        []string{"schema", "api", "contracts"},
			},
		},
		DefaultInputModes:  []string{"text/plain", "application/json"},
		DefaultOutputModes: []string{"text/markdown"},
	}

	sa := &SchemaAgent{}
	sa.BaseAgent = NewBaseAgent(card, sa.processMessage)
	return sa
}

// processMessage routes incoming messages to the appropriate skill handler.
func (sa *SchemaAgent) processMessage(_ context.Context, _ *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
	text := extractText(msg)
	skill := detectSchemaSkill(text)

	switch skill {
	case "translate-schema":
		return sa.handleTranslateSchema(text)
	case "validate-types":
		return sa.handleValidateTypes(text)
	case "write-contracts":
		return sa.handleWriteContracts(text)
	default:
		return nil, fmt.Errorf("unknown skill: could not determine skill from message text")
	}
}

// detectSchemaSkill determines which schema skill to invoke based on message text keywords.
func detectSchemaSkill(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "translate-schema") || strings.Contains(lower, "translate schema"):
		return "translate-schema"
	case strings.Contains(lower, "validate-types") || strings.Contains(lower, "validate types") || strings.Contains(lower, "validate"):
		return "validate-types"
	case strings.Contains(lower, "write-contracts") || strings.Contains(lower, "write contracts") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "api contract"):
		return "write-contracts"
	case strings.Contains(lower, "entity") || strings.Contains(lower, "struct") || strings.Contains(lower, "schema") || strings.Contains(lower, "type "):
		return "translate-schema"
	default:
		return ""
	}
}

// --- translate-schema skill ---

// entityPattern matches "Entity X with fields a (type), b (type)" style descriptions.
var entityPattern = regexp.MustCompile(`(?i)(?:entity|type)\s+(\w+)\s+(?:with\s+fields?\s+)?(.+)`)

// bracePattern matches "type X { a: type, b: type }" style descriptions.
var bracePattern = regexp.MustCompile(`(?i)type\s+(\w+)\s*\{\s*(.+?)\s*\}`)

// entityField represents a parsed field within an entity.
type entityField struct {
	Name string
	Type string
}

// handleTranslateSchema parses entity descriptions and generates Go structs.
func (sa *SchemaAgent) handleTranslateSchema(text string) ([]a2a.Artifact, error) {
	entities := parseEntities(text)
	if len(entities) == 0 {
		return nil, fmt.Errorf("translate-schema: no entity descriptions found in message")
	}

	var sb strings.Builder
	sb.WriteString("# Generated Go Structs\n\n```go\n")
	for i, e := range entities {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatStruct(e.name, e.fields))
	}
	sb.WriteString("```\n")

	return []a2a.Artifact{
		{
			ArtifactID:  "schema-structs",
			Name:        "Generated Structs",
			Description: "Go struct definitions generated from entity descriptions",
			Parts:       []a2a.Part{a2a.TextPart(sb.String())},
		},
	}, nil
}

// parsedEntity holds a parsed entity name and its fields.
type parsedEntity struct {
	name   string
	fields []entityField
}

// parseEntities extracts entity definitions from the input text.
func parseEntities(text string) []parsedEntity {
	var entities []parsedEntity

	// Try brace-style first: type X { a: type, b: type }
	for _, match := range bracePattern.FindAllStringSubmatch(text, -1) {
		name := match[1]
		fieldsStr := match[2]
		fields := parseBraceFields(fieldsStr)
		if len(fields) > 0 {
			entities = append(entities, parsedEntity{name: name, fields: fields})
		}
	}

	if len(entities) > 0 {
		return entities
	}

	// Try "Entity X with fields ..." style, split by newlines or semicolons.
	lines := splitLines(text)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if match := entityPattern.FindStringSubmatch(line); match != nil {
			name := match[1]
			fieldsStr := match[2]
			fields := parseEntityFields(fieldsStr)
			if len(fields) > 0 {
				entities = append(entities, parsedEntity{name: name, fields: fields})
			}
		}
	}

	return entities
}

// splitLines splits text by newlines and semicolons.
func splitLines(text string) []string {
	// First split by newlines, then by semicolons.
	var result []string
	for _, line := range strings.Split(text, "\n") {
		parts := strings.Split(line, ";")
		result = append(result, parts...)
	}
	return result
}

// parseBraceFields parses "a: type, b: type" field declarations.
func parseBraceFields(s string) []entityField {
	var fields []entityField
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Split on ":"
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		name := strings.TrimSpace(kv[0])
		typ := strings.TrimSpace(kv[1])
		if name == "" || typ == "" {
			continue
		}
		fields = append(fields, entityField{Name: name, Type: mapType(typ)})
	}
	return fields
}

// fieldPattern matches "name (type)" within entity field descriptions.
var fieldPattern = regexp.MustCompile(`(\w+)\s*\(([^)]+)\)`)

// parseEntityFields parses "a (type), b (type)" style field lists.
func parseEntityFields(s string) []entityField {
	var fields []entityField
	for _, match := range fieldPattern.FindAllStringSubmatch(s, -1) {
		name := strings.TrimSpace(match[1])
		typ := strings.TrimSpace(match[2])
		if name != "" && typ != "" {
			fields = append(fields, entityField{Name: name, Type: mapType(typ)})
		}
	}
	return fields
}

// mapType normalizes common type names to Go types.
func mapType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "string", "str", "text":
		return "string"
	case "int", "integer", "number":
		return "int"
	case "int64", "long":
		return "int64"
	case "int32":
		return "int32"
	case "float", "float64", "double", "decimal":
		return "float64"
	case "float32":
		return "float32"
	case "bool", "boolean":
		return "bool"
	case "time", "datetime", "timestamp":
		return "time.Time"
	case "[]string", "string list", "string array", "strings":
		return "[]string"
	case "[]int", "int list", "int array":
		return "[]int"
	case "[]byte", "bytes", "binary":
		return "[]byte"
	default:
		return t
	}
}

// exportName converts a field name to an exported Go identifier.
func exportName(s string) string {
	if s == "" {
		return s
	}
	// Handle snake_case by converting to PascalCase.
	parts := strings.Split(s, "_")
	var sb strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		sb.WriteString(string(runes))
	}
	result := sb.String()
	if result == "" {
		return s
	}
	// Handle common initialisms.
	upper := strings.ToUpper(result)
	switch upper {
	case "ID", "URL", "API", "HTTP", "JSON", "XML", "SQL", "HTML", "CSS", "IP", "UUID":
		return upper
	}
	return result
}

// jsonTag generates a camelCase JSON tag for a field name.
func jsonTag(s string) string {
	if s == "" {
		return s
	}
	// Handle snake_case first.
	parts := strings.Split(s, "_")
	var sb strings.Builder
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			sb.WriteString(strings.ToLower(part))
		} else {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			sb.WriteString(string(runes))
		}
	}
	result := sb.String()
	if result == "" {
		// If the name has no underscores, just lowercase the first rune.
		runes := []rune(s)
		runes[0] = unicode.ToLower(runes[0])
		return string(runes)
	}
	return result
}

// formatStruct generates a Go struct definition with JSON tags.
func formatStruct(name string, fields []entityField) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", exportName(name)))
	for _, f := range fields {
		exported := exportName(f.Name)
		tag := jsonTag(f.Name)
		sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", exported, f.Type, tag))
	}
	sb.WriteString("}\n")
	return sb.String()
}

// --- validate-types skill ---

// handleValidateTypes performs basic type validation. Full validation requires
// MCP tools which are not yet integrated.
func (sa *SchemaAgent) handleValidateTypes(text string) ([]a2a.Artifact, error) {
	// Check if code is provided for basic validation.
	if containsGoCode(text) {
		issues := basicSyntaxCheck(text)
		var sb strings.Builder
		sb.WriteString("# Type Validation Results\n\n")
		if len(issues) == 0 {
			sb.WriteString("No obvious issues found in the provided code.\n\n")
		} else {
			sb.WriteString("## Issues Found\n\n")
			for _, issue := range issues {
				sb.WriteString(fmt.Sprintf("- %s\n", issue))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("> **Note:** Full type validation requires MCP tools integration, ")
		sb.WriteString("which is not yet available. The above checks are basic syntax-level only.\n")

		return []a2a.Artifact{
			{
				ArtifactID:  "validation-results",
				Name:        "Validation Results",
				Description: "Basic syntax validation results",
				Parts:       []a2a.Part{a2a.TextPart(sb.String())},
			},
		}, nil
	}

	return []a2a.Artifact{
		{
			ArtifactID:  "validation-note",
			Name:        "Validation Note",
			Description: "Note about validation capabilities",
			Parts: []a2a.Part{a2a.TextPart(
				"# Type Validation\n\n" +
					"Full type validation requires MCP tools integration, which is not yet available.\n\n" +
					"To validate types, please provide Go code containing struct definitions, " +
					"and basic syntax checks will be performed.\n",
			)},
		},
	}, nil
}

// containsGoCode checks whether the text appears to contain Go code.
func containsGoCode(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "type ") && strings.Contains(lower, "struct") ||
		strings.Contains(text, "```go") ||
		strings.Contains(text, "func ")
}

// basicSyntaxCheck performs rudimentary checks on Go struct definitions.
func basicSyntaxCheck(text string) []string {
	var issues []string

	// Extract code from markdown code blocks if present.
	code := extractCodeBlock(text)

	// Check for unmatched braces.
	opens := strings.Count(code, "{")
	closes := strings.Count(code, "}")
	if opens != closes {
		issues = append(issues, fmt.Sprintf("unmatched braces: %d opening vs %d closing", opens, closes))
	}

	// Check for unexported struct fields in exported types.
	lines := strings.Split(code, "\n")
	inStruct := false
	exportedStruct := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			inStruct = true
			// Check if the type name is exported.
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				name := parts[1]
				exportedStruct = len(name) > 0 && unicode.IsUpper([]rune(name)[0])
			}
			continue
		}
		if trimmed == "}" {
			inStruct = false
			exportedStruct = false
			continue
		}
		if inStruct && exportedStruct && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				fieldName := fields[0]
				if len(fieldName) > 0 && unicode.IsLower([]rune(fieldName)[0]) {
					issues = append(issues, fmt.Sprintf("unexported field %q in exported struct", fieldName))
				}
			}
		}
	}

	return issues
}

// extractCodeBlock extracts content from a ```go ... ``` code block if present,
// otherwise returns the full text.
func extractCodeBlock(text string) string {
	start := strings.Index(text, "```go")
	if start == -1 {
		start = strings.Index(text, "```")
		if start == -1 {
			return text
		}
	}
	// Move past the opening fence line.
	start = strings.Index(text[start:], "\n")
	if start == -1 {
		return text
	}
	start += strings.Index(text, "```")
	// Adjust: start is relative, recalculate.
	fenceStart := strings.Index(text, "```")
	afterFenceLine := strings.Index(text[fenceStart:], "\n")
	if afterFenceLine == -1 {
		return text
	}
	codeStart := fenceStart + afterFenceLine + 1

	end := strings.Index(text[codeStart:], "```")
	if end == -1 {
		return text[codeStart:]
	}
	return text[codeStart : codeStart+end]
}

// --- write-contracts skill ---

// endpointGetPattern matches "endpoint GET /path returns Type" style.
var endpointGetPattern = regexp.MustCompile(
	`(?i)(?:endpoint\s+)?GET\s+(/\S+)\s+returns?\s+(\w+)(?:\s+list)?`,
)

// endpointPostPattern matches "POST /path takes Input returns Output" style.
var endpointPostPattern = regexp.MustCompile(
	`(?i)(?:endpoint\s+)?POST\s+(/\S+)\s+takes?\s+(\w+)\s+returns?\s+(\w+)`,
)

// endpointPutPattern matches "PUT /path takes Input returns Output" style.
var endpointPutPattern = regexp.MustCompile(
	`(?i)(?:endpoint\s+)?PUT\s+(/\S+)\s+takes?\s+(\w+)\s+returns?\s+(\w+)`,
)

// endpointDeletePattern matches "DELETE /path returns Type" or "DELETE /path" style.
var endpointDeletePattern = regexp.MustCompile(
	`(?i)(?:endpoint\s+)?DELETE\s+(/\S+)(?:\s+returns?\s+(\w+))?`,
)

// parsedEndpoint holds a parsed API endpoint description.
type parsedEndpoint struct {
	method     string
	path       string
	inputType  string
	outputType string
	isList     bool
}

// handleWriteContracts parses API endpoint descriptions and generates
// request/response struct pairs.
func (sa *SchemaAgent) handleWriteContracts(text string) ([]a2a.Artifact, error) {
	endpoints := parseEndpoints(text)
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("write-contracts: no API endpoint descriptions found in message")
	}

	var sb strings.Builder
	sb.WriteString("# Generated API Contracts\n\n```go\n")
	for i, ep := range endpoints {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatContract(ep))
	}
	sb.WriteString("```\n")

	return []a2a.Artifact{
		{
			ArtifactID:  "api-contracts",
			Name:        "API Contracts",
			Description: "Go request/response structs for API endpoints",
			Parts:       []a2a.Part{a2a.TextPart(sb.String())},
		},
	}, nil
}

// parseEndpoints extracts endpoint definitions from text.
func parseEndpoints(text string) []parsedEndpoint {
	var endpoints []parsedEndpoint

	lines := splitLines(text)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try POST pattern (most specific with input+output).
		if match := endpointPostPattern.FindStringSubmatch(line); match != nil {
			endpoints = append(endpoints, parsedEndpoint{
				method:     "POST",
				path:       match[1],
				inputType:  match[2],
				outputType: match[3],
			})
			continue
		}

		// Try PUT pattern.
		if match := endpointPutPattern.FindStringSubmatch(line); match != nil {
			endpoints = append(endpoints, parsedEndpoint{
				method:     "PUT",
				path:       match[1],
				inputType:  match[2],
				outputType: match[3],
			})
			continue
		}

		// Try GET pattern.
		if match := endpointGetPattern.FindStringSubmatch(line); match != nil {
			isList := strings.Contains(strings.ToLower(line), "list")
			endpoints = append(endpoints, parsedEndpoint{
				method:     "GET",
				path:       match[1],
				outputType: match[2],
				isList:     isList,
			})
			continue
		}

		// Try DELETE pattern.
		if match := endpointDeletePattern.FindStringSubmatch(line); match != nil {
			ep := parsedEndpoint{
				method: "DELETE",
				path:   match[1],
			}
			if match[2] != "" {
				ep.outputType = match[2]
			}
			endpoints = append(endpoints, ep)
			continue
		}
	}

	return endpoints
}

// contractName generates a struct name prefix from an endpoint path.
// For example, "/users" becomes "Users", "/users/{id}" becomes "UsersByID".
func contractName(method, path string) string {
	// Remove leading slash and split by /
	path = strings.TrimPrefix(path, "/")
	segments := strings.Split(path, "/")

	var parts []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		// Path parameters like {id} or :id
		if strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") {
			seg = strings.Trim(seg, "{}")
			seg = strings.TrimPrefix(seg, ":")
			parts = append(parts, "By"+exportName(seg))
		} else {
			parts = append(parts, exportName(seg))
		}
	}

	prefix := strings.Join(parts, "")
	if prefix == "" {
		prefix = "Root"
	}
	return method[0:1] + strings.ToLower(method[1:]) + prefix
}

// formatContract generates request/response struct pairs for an endpoint.
func formatContract(ep parsedEndpoint) string {
	var sb strings.Builder
	name := contractName(ep.method, ep.path)

	// Generate request struct.
	sb.WriteString(fmt.Sprintf("// %sRequest is the request for %s %s.\n", name, ep.method, ep.path))
	sb.WriteString(fmt.Sprintf("type %sRequest struct {\n", name))

	// Add path parameters from the path.
	pathParams := extractPathParams(ep.path)
	for _, param := range pathParams {
		exported := exportName(param)
		sb.WriteString(fmt.Sprintf("\t%s string `json:\"%s\"`\n", exported, jsonTag(param)))
	}

	// If there is an input type, embed it as Body.
	if ep.inputType != "" {
		sb.WriteString(fmt.Sprintf("\tBody %s `json:\"body\"`\n", exportName(ep.inputType)))
	}
	sb.WriteString("}\n\n")

	// Generate response struct.
	sb.WriteString(fmt.Sprintf("// %sResponse is the response for %s %s.\n", name, ep.method, ep.path))
	sb.WriteString(fmt.Sprintf("type %sResponse struct {\n", name))
	if ep.outputType != "" {
		outType := exportName(ep.outputType)
		if ep.isList {
			sb.WriteString(fmt.Sprintf("\tItems []%s `json:\"items\"`\n", outType))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s %s `json:\"%s\"`\n", outType, outType, jsonTag(ep.outputType)))
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

// extractPathParams extracts parameter names from a URL path.
// Recognizes both {param} and :param styles.
func extractPathParams(path string) []string {
	var params []string
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params = append(params, strings.Trim(seg, "{}"))
		} else if strings.HasPrefix(seg, ":") {
			params = append(params, strings.TrimPrefix(seg, ":"))
		}
	}
	return params
}
