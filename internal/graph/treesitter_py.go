package graph

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// pyExtractor extracts symbols and edges from Python source files.
type pyExtractor struct{}

func (e *pyExtractor) Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge) {
	var symbols []SymbolNode
	var edges []Edge

	cursor := root.Walk()
	defer cursor.Close()

	e.walk(cursor, source, filePath, &symbols, &edges)
	return symbols, edges
}

func (e *pyExtractor) walk(
	cursor *tree_sitter.TreeCursor,
	source []byte,
	filePath string,
	symbols *[]SymbolNode,
	edges *[]Edge,
) {
	node := cursor.Node()
	kind := node.Kind()

	switch kind {
	case "function_definition":
		if isPyTopLevel(node) {
			if sym := e.extractFunction(node, source, filePath); sym != nil {
				*symbols = append(*symbols, *sym)
			}
		}

	case "class_definition":
		if isPyTopLevel(node) {
			if sym := e.extractClass(node, source, filePath); sym != nil {
				*symbols = append(*symbols, *sym)
			}
		}

	case "decorated_definition":
		// The actual function_definition or class_definition is a child; we
		// handle it when we recurse. Skip the decorated_definition itself.

	case "import_statement":
		extracted := e.extractImport(node, source, filePath)
		*edges = append(*edges, extracted...)

	case "import_from_statement":
		if edge := e.extractFromImport(node, source, filePath); edge != nil {
			*edges = append(*edges, *edge)
		}

	case "call":
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

func (e *pyExtractor) extractFunction(node *tree_sitter.Node, source []byte, filePath string) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	return &SymbolNode{
		Name:      name,
		Kind:      SymbolKindFunction,
		Exported:  isPyExported(name),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func (e *pyExtractor) extractClass(node *tree_sitter.Node, source []byte, filePath string) *SymbolNode {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := nameNode.Utf8Text(source)
	return &SymbolNode{
		Name:      name,
		Kind:      SymbolKindClass,
		Exported:  isPyExported(name),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

func (e *pyExtractor) extractImport(node *tree_sitter.Node, source []byte, filePath string) []Edge {
	var edges []Edge
	// import_statement children: "import" keyword then dotted_name(s).
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Kind() == "dotted_name" {
			moduleName := child.Utf8Text(source)
			if moduleName != "" {
				edges = append(edges, Edge{
					SourceID: filePath,
					TargetID: moduleName,
					Kind:     EdgeKindImports,
				})
			}
		}
	}
	return edges
}

func (e *pyExtractor) extractFromImport(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	moduleNode := node.ChildByFieldName("module_name")
	if moduleNode == nil {
		// Fall back: look for a dotted_name child.
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "dotted_name" {
				moduleNode = child
				break
			}
		}
	}
	if moduleNode == nil {
		return nil
	}

	moduleName := moduleNode.Utf8Text(source)
	if moduleName == "" {
		return nil
	}

	return &Edge{
		SourceID: filePath,
		TargetID: moduleName,
		Kind:     EdgeKindImports,
	}
}

func (e *pyExtractor) extractCall(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	fnNode := node.ChildByFieldName("function")
	if fnNode == nil {
		return nil
	}

	var callee string
	switch fnNode.Kind() {
	case "identifier":
		callee = fnNode.Utf8Text(source)
	case "attribute":
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

// isPyTopLevel returns true if the node is at the module top level.
// A top-level node has a parent that is "module", or a parent that is
// "decorated_definition" whose own parent is "module".
func isPyTopLevel(node *tree_sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	if parent.Kind() == "module" {
		return true
	}
	if parent.Kind() == "decorated_definition" {
		grandparent := parent.Parent()
		return grandparent != nil && grandparent.Kind() == "module"
	}
	return false
}

// isPyExported returns true if the name does not start with an underscore.
func isPyExported(name string) bool {
	return !strings.HasPrefix(name, "_")
}
