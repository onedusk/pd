package graph

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// tsExtractor extracts symbols and edges from TypeScript source files.
type tsExtractor struct{}

func (e *tsExtractor) Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge) {
	var symbols []SymbolNode
	var edges []Edge

	cursor := root.Walk()
	defer cursor.Close()

	e.walk(cursor, source, filePath, &symbols, &edges)
	return symbols, edges
}

func (e *tsExtractor) walk(
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
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindFunction); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "class_declaration":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindClass); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "interface_declaration":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindInterface); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "type_alias_declaration":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindType); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "enum_declaration":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindEnum); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "lexical_declaration":
		extracted := e.extractArrowFunctions(node, source, filePath)
		*symbols = append(*symbols, extracted...)

	case "import_statement":
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

// extractNamedSymbol extracts a symbol from a node that has a "name" field child.
func (e *tsExtractor) extractNamedSymbol(
	node *tree_sitter.Node,
	source []byte,
	filePath string,
	symbolKind SymbolKind,
) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	exported := isTSExported(node)

	return &SymbolNode{
		Name:      name,
		Kind:      symbolKind,
		Exported:  exported,
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

// extractArrowFunctions looks for arrow function expressions inside a
// lexical_declaration (e.g., "const foo = () => { ... }").
func (e *tsExtractor) extractArrowFunctions(node *tree_sitter.Node, source []byte, filePath string) []SymbolNode {
	var result []SymbolNode
	exported := isTSExported(node)

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || child.Kind() != "variable_declarator" {
			continue
		}

		valueNode := child.ChildByFieldName("value")
		if valueNode == nil {
			continue
		}
		if valueNode.Kind() != "arrow_function" {
			continue
		}

		nameNode := child.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		name := nameNode.Utf8Text(source)

		result = append(result, SymbolNode{
			Name:      name,
			Kind:      SymbolKindFunction,
			Exported:  exported,
			FilePath:  filePath,
			StartLine: int(child.StartPosition().Row) + 1,
			EndLine:   int(child.EndPosition().Row) + 1,
		})
	}
	return result
}

func (e *tsExtractor) extractImport(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	sourceNode := node.ChildByFieldName("source")
	if sourceNode == nil {
		// Fall back: look for a string child.
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "string" {
				sourceNode = child
				break
			}
		}
	}
	if sourceNode == nil {
		return nil
	}

	importPath := strings.Trim(sourceNode.Utf8Text(source), "\"'`")
	if importPath == "" {
		return nil
	}

	return &Edge{
		SourceID: filePath,
		TargetID: importPath,
		Kind:     EdgeKindImports,
	}
}

func (e *tsExtractor) extractCall(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	fnNode := node.ChildByFieldName("function")
	if fnNode == nil {
		return nil
	}

	var callee string
	switch fnNode.Kind() {
	case "identifier":
		callee = fnNode.Utf8Text(source)
	case "member_expression":
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

// isTSExported checks if a node is exported by looking at whether its parent
// is an export_statement.
func isTSExported(node *tree_sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	return parent.Kind() == "export_statement"
}
