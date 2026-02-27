package graph

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// rsExtractor extracts symbols and edges from Rust source files.
type rsExtractor struct{}

func (e *rsExtractor) Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge) {
	var symbols []SymbolNode
	var edges []Edge

	cursor := root.Walk()
	defer cursor.Close()

	e.walk(cursor, source, filePath, &symbols, &edges)
	return symbols, edges
}

func (e *rsExtractor) walk(
	cursor *tree_sitter.TreeCursor,
	source []byte,
	filePath string,
	symbols *[]SymbolNode,
	edges *[]Edge,
) {
	node := cursor.Node()
	kind := node.Kind()

	switch kind {
	case "function_item":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindFunction); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "struct_item":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindType); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "enum_item":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindEnum); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "trait_item":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindInterface); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "type_item":
		if sym := e.extractNamedSymbol(node, source, filePath, SymbolKindType); sym != nil {
			*symbols = append(*symbols, *sym)
		}

	case "impl_item":
		e.extractImpl(node, source, filePath, symbols, edges)

	case "use_declaration":
		if edge := e.extractUse(node, source, filePath); edge != nil {
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
func (e *rsExtractor) extractNamedSymbol(
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
	return &SymbolNode{
		Name:      name,
		Kind:      symbolKind,
		Exported:  isRustPub(node),
		FilePath:  filePath,
		StartLine: int(node.StartPosition().Row) + 1,
		EndLine:   int(node.EndPosition().Row) + 1,
	}
}

// extractImpl processes an impl_item: extracts methods inside, and detects
// trait implementations to produce EdgeKindImplements edges.
func (e *rsExtractor) extractImpl(
	node *tree_sitter.Node,
	source []byte,
	filePath string,
	symbols *[]SymbolNode,
	edges *[]Edge,
) {
	// Check for trait impl: "impl Trait for Type"
	traitNode := node.ChildByFieldName("trait")
	typeNode := node.ChildByFieldName("type")

	if traitNode != nil && typeNode != nil {
		traitName := traitNode.Utf8Text(source)
		typeName := typeNode.Utf8Text(source)
		if traitName != "" && typeName != "" {
			*edges = append(*edges, Edge{
				SourceID: typeName,
				TargetID: traitName,
				Kind:     EdgeKindImplements,
			})
		}
	}

	// Extract methods inside the impl body.
	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		return
	}

	for i := uint(0); i < bodyNode.ChildCount(); i++ {
		child := bodyNode.Child(i)
		if child == nil || child.Kind() != "function_item" {
			continue
		}
		nameNode := child.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		name := nameNode.Utf8Text(source)
		*symbols = append(*symbols, SymbolNode{
			Name:      name,
			Kind:      SymbolKindMethod,
			Exported:  isRustPub(child),
			FilePath:  filePath,
			StartLine: int(child.StartPosition().Row) + 1,
			EndLine:   int(child.EndPosition().Row) + 1,
		})
	}
}

func (e *rsExtractor) extractUse(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	// The use_declaration's argument is typically a scoped_identifier, use_wildcard,
	// or use_list. We extract the full text as the import path.
	argNode := node.ChildByFieldName("argument")
	if argNode == nil {
		// Fall back to getting all text after "use " keyword.
		text := node.Utf8Text(source)
		if text == "" {
			return nil
		}
		return &Edge{
			SourceID: filePath,
			TargetID: text,
			Kind:     EdgeKindImports,
		}
	}

	importPath := argNode.Utf8Text(source)
	if importPath == "" {
		return nil
	}

	return &Edge{
		SourceID: filePath,
		TargetID: importPath,
		Kind:     EdgeKindImports,
	}
}

func (e *rsExtractor) extractCall(node *tree_sitter.Node, source []byte, filePath string) *Edge {
	fnNode := node.ChildByFieldName("function")
	if fnNode == nil {
		return nil
	}

	var callee string
	switch fnNode.Kind() {
	case "identifier":
		callee = fnNode.Utf8Text(source)
	case "scoped_identifier":
		callee = fnNode.Utf8Text(source)
	case "field_expression":
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

// isRustPub checks if a node has a visibility_modifier child with "pub" text.
func isRustPub(node *tree_sitter.Node) bool {
	if node.ChildCount() == 0 {
		return false
	}
	first := node.Child(0)
	if first == nil {
		return false
	}
	return first.Kind() == "visibility_modifier"
}
