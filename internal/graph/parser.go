package graph

import "context"

// ParseResult holds the extracted symbols and edges from a single file.
type ParseResult struct {
	File    FileNode     `json:"file"`
	Symbols []SymbolNode `json:"symbols"`
	Edges   []Edge       `json:"edges"` // DEFINES, IMPORTS, CALLS edges
}

// Parser extracts structural information from source files.
// Implementations: TreeSitterParser (production), StubParser (testing).
type Parser interface {
	// Parse extracts symbols and relationships from a single source file.
	// source is the file content. lang determines which grammar to use.
	Parse(ctx context.Context, path string, source []byte, lang Language) (*ParseResult, error)

	// SupportedLanguages returns the languages this parser can handle.
	SupportedLanguages() []Language

	// Close releases parser resources (Tree-sitter C memory).
	Close() error
}
