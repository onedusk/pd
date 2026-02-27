package graph

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// goExtractor extracts symbols and edges from Go source files.
type goExtractor struct{}

func (e *goExtractor) Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge) {
	var symbols []SymbolNode
	var edges []Edge

	cursor := root.Walk()
	defer cursor.Close()

	e.walk(cursor, source, filePath, &symbols, &edges)
	return symbols, edges
}

func (e *goExtractor) walk(
	cursor *tree_sitter.TreeCursor,
	source []byte,
	filePath string,
	symbols *[]SymbolNode,
	edges *[]Edge,
) {
	node := cursor.Node()
	kind := node.Kind()

	switch kind {
	case "function_declaration":
		if sym := e.extractFunction(node, source, filePath); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "method_declaration":
		if sym := e.extractMethod(node, source, filePath); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "type_declaration":
		extracted := e.extractTypeDeclaration(node, source, filePath)
		*symbols = append(*symbols, extracted...)

	case "import_spec":
		if edge := e.extractImport(node, source, filePath); edge != nil {
			*edges = append(*edges, *edge)
		}

	case "call_expression":
		if edge := e.extractCall(node, source, filePath); edge != nil {
			*edges = append(*edges, *edge)
		}
	}

	if cursor.GotoFirstChild() {
		e.walk(cursor, source, filePath, symbols, edges)
		for cursor.GotoNextSibling() {
			e.walk(cursor, source, filePath, symbols, edges)
		}
		cursor.GotoParent()
	}
}

func (e *goExtractor) extractFunction(node *tree_sitter.Node, source []byte, filePath string) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	return &SymbolNode{
		Name:      name,
		Kind:      SymbolKindFunction,
		Exported:  isGoExported(name),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func (e *goExtractor) extractMethod(node *tree_sitter.Node, source []byte, filePath string) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	return &SymbolNode{
		Name:      name,
		Kind:      SymbolKindMethod,
		Exported:  isGoExported(name),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func (e *goExtractor) extractTypeDeclaration(node *tree_sitter.Node, source []byte, filePath string) []SymbolNode {
	var result []SymbolNode

	// type_declaration contains one or more type_spec children.
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() != "type_spec" {
			continue
		}
		sym := e.extractTypeSpec(child, source, filePath)
		if sym != nil {
			result = append(result, *sym)
		}
	}
	return result
}

func (e *goExtractor) extractTypeSpec(node *tree_sitter.Node, source []byte, filePath string) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)

	symbolKind := SymbolKindType
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		switch typeNode.Kind() {
		case "interface_type":
			symbolKind = SymbolKindInterface
		case "struct_type":
			symbolKind = SymbolKindType
		}
	}

	return &SymbolNode{
		Name:      name,
		Kind:      symbolKind,
		Exported:  isGoExported(name),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func (e *goExtractor) extractImport(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	pathNode := node.ChildByFieldName("path")
	if pathNode == nil {
		// Fall back to finding an interpreted_string_literal child.
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "interpreted_string_literal" {
				pathNode = child
				break
			}
		}
	}
	if pathNode == nil {
		return nil
	}

	importPath := strings.Trim(pathNode.Utf8Text(source), "\"")
	if importPath == "" {
		return nil
	}

	return &Edge{
		SourceID: filePath,
		TargetID: importPath,
		Kind:     EdgeKindImports,
	}
}

func (e *goExtractor) extractCall(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	fnNode := node.ChildByFieldName("function")
	if fnNode == nil {
		return nil
	}

	// Best-effort: only extract simple identifiers and selector expressions.
	var callee string
	switch fnNode.Kind() {
	case "identifier":
		callee = fnNode.Utf8Text(source)
	case "selector_expression":
		callee = fnNode.Utf8Text(source)
	default:
		return nil
	}

	if callee == "" {
		return nil
	}

	return &Edge{
		SourceID: filePath,
		TargetID: callee,
		Kind:     EdgeKindCalls,
	}
}

// isGoExported returns true if the first rune of name is an uppercase letter.
func isGoExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
