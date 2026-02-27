package graph

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findSymbol returns the first SymbolNode whose Name matches, or nil.
func findSymbol(symbols []SymbolNode, name string) *SymbolNode {
	for i := range symbols {
		if symbols[i].Name == name {
			return &symbols[i]
		}
	}
	return nil
}

// findEdgesByKind returns all edges matching the given kind.
func findEdgesByKind(edges []Edge, kind EdgeKind) []Edge {
	var out []Edge
	for _, e := range edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// readFixture reads a test fixture file relative to the project root.
// Tests run from internal/graph/, so the relative path is ../../testdata/...
func readFixture(t *testing.T, relPath string) []byte {
	t.Helper()
	data, err := os.ReadFile("../../" + relPath)
	require.NoError(t, err, "reading fixture %s", relPath)
	return data
}

// assertLineRange checks that StartLine and EndLine are populated and valid.
func assertLineRange(t *testing.T, sym *SymbolNode) {
	t.Helper()
	assert.Greater(t, sym.StartLine, 0, "StartLine should be > 0 for %s", sym.Name)
	assert.Greater(t, sym.EndLine, 0, "EndLine should be > 0 for %s", sym.Name)
	assert.LessOrEqual(t, sym.StartLine, sym.EndLine, "StartLine <= EndLine for %s", sym.Name)
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_SupportedLanguages
// ---------------------------------------------------------------------------

func TestTreeSitterParser_SupportedLanguages(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()

	langs := p.SupportedLanguages()
	assert.Len(t, langs, 4, "should support exactly 4 languages")

	langSet := make(map[Language]bool, len(langs))
	for _, l := range langs {
		langSet[l] = true
	}
	assert.True(t, langSet[LangGo], "should support Go")
	assert.True(t, langSet[LangTypeScript], "should support TypeScript")
	assert.True(t, langSet[LangPython], "should support Python")
	assert.True(t, langSet[LangRust], "should support Rust")
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_Go
// ---------------------------------------------------------------------------

func TestTreeSitterParser_Go(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	t.Run("model.go", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/go_project/model.go")
		res, err := p.Parse(ctx, "model.go", src, LangGo)
		require.NoError(t, err)
		require.NotNil(t, res)

		// FileNode
		assert.Equal(t, "model.go", res.File.Path)
		assert.Equal(t, LangGo, res.File.Language)
		assert.Greater(t, res.File.LOC, 0)

		// Symbols: User (struct/type), Repository (interface), newUser (function)
		assert.GreaterOrEqual(t, len(res.Symbols), 3, "expected at least 3 symbols")

		user := findSymbol(res.Symbols, "User")
		require.NotNil(t, user, "User symbol should exist")
		assert.Equal(t, SymbolKindType, user.Kind)
		assert.True(t, user.Exported)
		assertLineRange(t, user)

		repo := findSymbol(res.Symbols, "Repository")
		require.NotNil(t, repo, "Repository symbol should exist")
		assert.Equal(t, SymbolKindInterface, repo.Kind)
		assert.True(t, repo.Exported)
		assertLineRange(t, repo)

		newUser := findSymbol(res.Symbols, "newUser")
		require.NotNil(t, newUser, "newUser symbol should exist")
		assert.Equal(t, SymbolKindFunction, newUser.Kind)
		assert.False(t, newUser.Exported)
		assertLineRange(t, newUser)

		// No imports in model.go
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		assert.Empty(t, imports, "model.go has no imports")
	})

	t.Run("service.go", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/go_project/service.go")
		res, err := p.Parse(ctx, "service.go", src, LangGo)
		require.NoError(t, err)
		require.NotNil(t, res)

		// Symbols: UserService (type), NewUserService (func), GetUser (method), CreateUser (method)
		assert.GreaterOrEqual(t, len(res.Symbols), 4, "expected at least 4 symbols")

		us := findSymbol(res.Symbols, "UserService")
		require.NotNil(t, us, "UserService symbol should exist")
		assert.Equal(t, SymbolKindType, us.Kind)
		assert.True(t, us.Exported)

		nus := findSymbol(res.Symbols, "NewUserService")
		require.NotNil(t, nus, "NewUserService symbol should exist")
		assert.Equal(t, SymbolKindFunction, nus.Kind)
		assert.True(t, nus.Exported)

		gu := findSymbol(res.Symbols, "GetUser")
		require.NotNil(t, gu, "GetUser symbol should exist")
		assert.Equal(t, SymbolKindMethod, gu.Kind)
		assert.True(t, gu.Exported)

		cu := findSymbol(res.Symbols, "CreateUser")
		require.NotNil(t, cu, "CreateUser symbol should exist")
		assert.Equal(t, SymbolKindMethod, cu.Kind)
		assert.True(t, cu.Exported)

		// Import edge for "fmt"
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 1, "should have at least 1 import edge")
		found := false
		for _, e := range imports {
			if e.TargetID == "fmt" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have import edge for fmt")

		// At least one call edge
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})

	t.Run("main.go", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/go_project/main.go")
		res, err := p.Parse(ctx, "main.go", src, LangGo)
		require.NoError(t, err)
		require.NotNil(t, res)

		run := findSymbol(res.Symbols, "Run")
		require.NotNil(t, run, "Run symbol should exist")
		assert.Equal(t, SymbolKindFunction, run.Kind)
		assert.True(t, run.Exported)
		assertLineRange(t, run)

		// Import edge for "fmt"
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 1)
		found := false
		for _, e := range imports {
			if e.TargetID == "fmt" {
				found = true
				break
			}
		}
		assert.True(t, found, "should import fmt")

		// At least one call edge (fmt.Println)
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_TypeScript
// ---------------------------------------------------------------------------

func TestTreeSitterParser_TypeScript(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	t.Run("types.ts", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/ts_project/types.ts")
		res, err := p.Parse(ctx, "types.ts", src, LangTypeScript)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, LangTypeScript, res.File.Language)
		assert.Greater(t, res.File.LOC, 0)

		// Symbols: User (interface), UserRole (type), Status (enum), validateEmail (function)
		assert.GreaterOrEqual(t, len(res.Symbols), 4, "expected at least 4 symbols")

		user := findSymbol(res.Symbols, "User")
		require.NotNil(t, user, "User interface should exist")
		assert.Equal(t, SymbolKindInterface, user.Kind)
		assert.True(t, user.Exported, "User is export-ed")
		assertLineRange(t, user)

		role := findSymbol(res.Symbols, "UserRole")
		require.NotNil(t, role, "UserRole type should exist")
		assert.Equal(t, SymbolKindType, role.Kind)
		assert.True(t, role.Exported)

		status := findSymbol(res.Symbols, "Status")
		require.NotNil(t, status, "Status enum should exist")
		assert.Equal(t, SymbolKindEnum, status.Kind)
		assert.True(t, status.Exported)

		validate := findSymbol(res.Symbols, "validateEmail")
		require.NotNil(t, validate, "validateEmail function should exist")
		assert.Equal(t, SymbolKindFunction, validate.Kind)
		// validateEmail is declared as a plain function_declaration (not inside
		// an export_statement), so isTSExported returns false. The
		// "export default validateEmail" is a separate statement that re-exports
		// the identifier; the extractor does not mark the original declaration.
		assert.False(t, validate.Exported, "validateEmail function_declaration is not inside an export_statement")
	})

	t.Run("service.ts", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/ts_project/service.ts")
		res, err := p.Parse(ctx, "service.ts", src, LangTypeScript)
		require.NoError(t, err)
		require.NotNil(t, res)

		us := findSymbol(res.Symbols, "UserService")
		require.NotNil(t, us, "UserService class should exist")
		assert.Equal(t, SymbolKindClass, us.Kind)
		assert.True(t, us.Exported)
		assertLineRange(t, us)

		// Import edge targeting "./types"
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 1)
		found := false
		for _, e := range imports {
			if e.TargetID == "./types" {
				found = true
				break
			}
		}
		assert.True(t, found, "should import ./types")

		// At least one call edge (e.g., this.users.find, this.users.push)
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})

	t.Run("index.ts", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/ts_project/index.ts")
		res, err := p.Parse(ctx, "index.ts", src, LangTypeScript)
		require.NoError(t, err)
		require.NotNil(t, res)

		main := findSymbol(res.Symbols, "main")
		require.NotNil(t, main, "main function should exist")
		assert.Equal(t, SymbolKindFunction, main.Kind)
		assert.True(t, main.Exported)

		// Import edges: "./service" and "./types"
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 2)

		targets := make(map[string]bool)
		for _, e := range imports {
			targets[e.TargetID] = true
		}
		assert.True(t, targets["./service"], "should import ./service")
		assert.True(t, targets["./types"], "should import ./types")

		// At least one call edge (validateEmail, service.create)
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_Python
// ---------------------------------------------------------------------------

func TestTreeSitterParser_Python(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	t.Run("models.py", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/py_project/models.py")
		res, err := p.Parse(ctx, "models.py", src, LangPython)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, LangPython, res.File.Language)
		assert.Greater(t, res.File.LOC, 0)

		// Top-level symbols: User (class), _generate_id (function), create_user (function)
		assert.GreaterOrEqual(t, len(res.Symbols), 3, "expected at least 3 symbols")

		user := findSymbol(res.Symbols, "User")
		require.NotNil(t, user, "User class should exist")
		assert.Equal(t, SymbolKindClass, user.Kind)
		assert.True(t, user.Exported)
		assertLineRange(t, user)

		genID := findSymbol(res.Symbols, "_generate_id")
		require.NotNil(t, genID, "_generate_id function should exist")
		assert.Equal(t, SymbolKindFunction, genID.Kind)
		assert.False(t, genID.Exported, "underscore-prefixed names are unexported in Python")

		createUser := findSymbol(res.Symbols, "create_user")
		require.NotNil(t, createUser, "create_user function should exist")
		assert.Equal(t, SymbolKindFunction, createUser.Kind)
		assert.True(t, createUser.Exported)

		// At least one call edge (e.g., User(...), _generate_id())
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})

	t.Run("service.py", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/py_project/service.py")
		res, err := p.Parse(ctx, "service.py", src, LangPython)
		require.NoError(t, err)
		require.NotNil(t, res)

		us := findSymbol(res.Symbols, "UserService")
		require.NotNil(t, us, "UserService class should exist")
		assert.Equal(t, SymbolKindClass, us.Kind)
		assert.True(t, us.Exported)
		assertLineRange(t, us)

		// Import edge from "from .models import ..."
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 1, "should have at least 1 import edge")

		// At least one call edge (e.g., create_user(...))
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})

	t.Run("__init__.py", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/py_project/__init__.py")
		res, err := p.Parse(ctx, "__init__.py", src, LangPython)
		require.NoError(t, err)
		require.NotNil(t, res)

		// __init__.py has no top-level function or class definitions
		assert.Empty(t, res.Symbols, "__init__.py should have no symbols")

		// But it does have import edges (from .models, from .service)
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		assert.GreaterOrEqual(t, len(imports), 1, "should have import edges")
	})
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_Rust
// ---------------------------------------------------------------------------

func TestTreeSitterParser_Rust(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	t.Run("model.rs", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/rs_project/model.rs")
		res, err := p.Parse(ctx, "model.rs", src, LangRust)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, LangRust, res.File.Language)
		assert.Greater(t, res.File.LOC, 0)

		// Symbols: User (type/struct, pub), Repository (interface/trait, pub),
		// new (method, pub), validate_email (method, not pub)
		assert.GreaterOrEqual(t, len(res.Symbols), 4, "expected at least 4 symbols")

		user := findSymbol(res.Symbols, "User")
		require.NotNil(t, user, "User struct should exist")
		assert.Equal(t, SymbolKindType, user.Kind)
		assert.True(t, user.Exported)
		assertLineRange(t, user)

		repo := findSymbol(res.Symbols, "Repository")
		require.NotNil(t, repo, "Repository trait should exist")
		assert.Equal(t, SymbolKindInterface, repo.Kind)
		assert.True(t, repo.Exported)
		assertLineRange(t, repo)

		newFn := findSymbol(res.Symbols, "new")
		require.NotNil(t, newFn, "new method should exist")
		assert.Equal(t, SymbolKindMethod, newFn.Kind)
		assert.True(t, newFn.Exported, "new is pub")

		validateEmail := findSymbol(res.Symbols, "validate_email")
		require.NotNil(t, validateEmail, "validate_email method should exist")
		assert.Equal(t, SymbolKindMethod, validateEmail.Kind)
		assert.False(t, validateEmail.Exported, "validate_email is not pub")
	})

	t.Run("service.rs", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/rs_project/service.rs")
		res, err := p.Parse(ctx, "service.rs", src, LangRust)
		require.NoError(t, err)
		require.NotNil(t, res)

		us := findSymbol(res.Symbols, "UserService")
		require.NotNil(t, us, "UserService struct should exist")
		assert.Equal(t, SymbolKindType, us.Kind)
		assert.True(t, us.Exported)
		assertLineRange(t, us)

		// Impl methods: new, get_user, create_user
		newMethod := findSymbol(res.Symbols, "new")
		require.NotNil(t, newMethod, "new method should exist")
		assert.Equal(t, SymbolKindMethod, newMethod.Kind)
		assert.True(t, newMethod.Exported)

		getUser := findSymbol(res.Symbols, "get_user")
		require.NotNil(t, getUser, "get_user method should exist")
		assert.Equal(t, SymbolKindMethod, getUser.Kind)
		assert.True(t, getUser.Exported)

		createUser := findSymbol(res.Symbols, "create_user")
		require.NotNil(t, createUser, "create_user method should exist")
		assert.Equal(t, SymbolKindMethod, createUser.Kind)
		assert.True(t, createUser.Exported)

		// Import edge from use declaration
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		require.GreaterOrEqual(t, len(imports), 1, "should have at least 1 import edge")

		// At least one call edge (e.g., self.repo.find_by_id, User::new, self.repo.save)
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})

	t.Run("main.rs", func(t *testing.T) {
		src := readFixture(t, "testdata/fixtures/rs_project/main.rs")
		res, err := p.Parse(ctx, "main.rs", src, LangRust)
		require.NoError(t, err)
		require.NotNil(t, res)

		main := findSymbol(res.Symbols, "main")
		require.NotNil(t, main, "main function should exist")
		assert.Equal(t, SymbolKindFunction, main.Kind)
		assert.False(t, main.Exported, "main is not pub")
		assertLineRange(t, main)

		// Import edges from use declaration
		imports := findEdgesByKind(res.Edges, EdgeKindImports)
		assert.GreaterOrEqual(t, len(imports), 1, "should have at least 1 import edge")

		// At least one call edge (User::new, "Alice".to_string, println!)
		// Note: println! is a macro and may not be captured as a call_expression.
		// User::new should be captured.
		calls := findEdgesByKind(res.Edges, EdgeKindCalls)
		assert.GreaterOrEqual(t, len(calls), 1, "should have at least 1 call edge")
	})
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_UnsupportedLanguage
// ---------------------------------------------------------------------------

func TestTreeSitterParser_UnsupportedLanguage(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	_, err := p.Parse(ctx, "test.rb", []byte("puts 'hello'"), Language("ruby"))
	require.Error(t, err, "parsing with an unsupported language should return an error")
	assert.Contains(t, err.Error(), "unsupported language")
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_EmptyFile
// ---------------------------------------------------------------------------

func TestTreeSitterParser_EmptyFile(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()
	ctx := context.Background()

	for _, lang := range []Language{LangGo, LangTypeScript, LangPython, LangRust} {
		t.Run(string(lang), func(t *testing.T) {
			res, err := p.Parse(ctx, "empty."+string(lang), []byte(""), lang)
			require.NoError(t, err, "parsing an empty file should not return an error")
			require.NotNil(t, res)
			assert.Empty(t, res.Symbols, "empty file should produce 0 symbols")
			assert.Equal(t, 0, res.File.LOC, "empty file LOC should be 0")
		})
	}
}

// ---------------------------------------------------------------------------
// TestTreeSitterParser_Close
// ---------------------------------------------------------------------------

func TestTreeSitterParser_Close(t *testing.T) {
	p := NewTreeSitterParser()
	err := p.Close()
	assert.NoError(t, err, "Close should not return an error")

	// Calling Close a second time should also be safe.
	err = p.Close()
	assert.NoError(t, err, "second Close should also not return an error")
}
