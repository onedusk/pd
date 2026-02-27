package graph

import (
	"bytes"
	"context"
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// extractor extracts symbols and edges from a parsed tree-sitter AST.
type extractor interface {
	Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge)
}

// TreeSitterParser implements the Parser interface using tree-sitter grammars.
// A new tree-sitter parser is created per Parse call, so this type is safe for
// sequential use but individual Parse calls are not thread-safe.
type TreeSitterParser struct {
	languages  map[Language]*tree_sitter.Language
	extractors map[Language]extractor
}

// NewTreeSitterParser creates a TreeSitterParser with Go, TypeScript, Python,
// and Rust grammars registered.
func NewTreeSitterParser() *TreeSitterParser {
	langs := map[Language]*tree_sitter.Language{
		LangGo:         tree_sitter.NewLanguage(tree_sitter_go.Language()),
		LangTypeScript: tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()),
		LangPython:     tree_sitter.NewLanguage(tree_sitter_python.Language()),
		LangRust:       tree_sitter.NewLanguage(tree_sitter_rust.Language()),
	}

	extractors := map[Language]extractor{
		LangGo:         &goExtractor{},
		LangTypeScript: &tsExtractor{},
		LangPython:     &pyExtractor{},
		LangRust:       &rsExtractor{},
	}

	return &TreeSitterParser{
		languages:  langs,
		extractors: extractors,
	}
}

// Parse extracts symbols and relationships from a single source file.
func (p *TreeSitterParser) Parse(_ context.Context, path string, source []byte, lang Language) (*ParseResult, error) {
	tsLang, ok := p.languages[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	ext, ok := p.extractors[lang]
	if !ok {
		return nil, fmt.Errorf("no extractor for language: %s", lang)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tsLang); err != nil {
		return nil, fmt.Errorf("set language %s: %w", lang, err)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter returned nil tree for %s", path)
	}
	defer tree.Close()

	root := tree.RootNode()
	symbols, edges := ext.Extract(root, source, path)

	loc := countLOC(source)

	return &ParseResult{
		File: FileNode{
			Path:     path,
			Language: lang,
			LOC:      loc,
		},
		Symbols: symbols,
		Edges:   edges,
	}, nil
}

// SupportedLanguages returns the languages this parser can handle.
func (p *TreeSitterParser) SupportedLanguages() []Language {
	langs := make([]Language, 0, len(p.languages))
	for l := range p.languages {
		langs = append(langs, l)
	}
	return langs
}

// Close is a no-op because parsers are created per Parse call.
func (p *TreeSitterParser) Close() error {
	return nil
}

// countLOC counts the number of lines in source by counting newline bytes
// and adding one for the final line if the source is non-empty.
func countLOC(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	return bytes.Count(source, []byte{'\n'}) + 1
}
