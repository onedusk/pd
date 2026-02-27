package graph

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Resolver rewrites raw import specifiers (extracted by tree-sitter) into
// repo-relative file paths that match FileNode.Path values. It is built once
// per build_graph call with the set of known file paths and any workspace
// metadata discovered in the repository root.
type Resolver struct {
	repoRoot     string
	fileSet      map[string]bool
	dirIndex     map[string][]string
	tsWorkspaces map[string]*tsWorkspace
	goModPath    string
}

// tsWorkspace holds metadata about a single npm/bun workspace package.
type tsWorkspace struct {
	dir            string            // repo-relative directory (e.g. "packages/db")
	mainFile       string            // default export target, repo-relative
	subpathExports map[string]string // "./queries" → "packages/db/src/queries.ts"
}

// NewResolver builds a Resolver from the repository root and the set of
// known repo-relative file paths. It scans for workspace metadata
// (package.json, go.mod) to enable package-aware resolution.
func NewResolver(repoRoot string, knownFiles []string) *Resolver {
	r := &Resolver{
		repoRoot:     repoRoot,
		fileSet:      make(map[string]bool, len(knownFiles)),
		dirIndex:     make(map[string][]string),
		tsWorkspaces: make(map[string]*tsWorkspace),
	}

	for _, f := range knownFiles {
		r.fileSet[f] = true
		dir := filepath.Dir(f)
		r.dirIndex[dir] = append(r.dirIndex[dir], f)
	}

	r.scanTSWorkspaces()
	r.scanGoMod()

	return r
}

// ResolveEdge attempts to resolve a single IMPORTS edge's TargetID from a raw
// import specifier to a repo-relative file path. Returns the resolved edge and
// true on success. Non-IMPORTS edges pass through unchanged.
func (r *Resolver) ResolveEdge(edge Edge, lang Language) (Edge, bool) {
	if edge.Kind != EdgeKindImports {
		return edge, true
	}

	var resolved string
	var ok bool

	switch lang {
	case LangTypeScript:
		resolved, ok = r.resolveTS(edge.TargetID, edge.SourceID)
	case LangGo:
		resolved, ok = r.resolveGo(edge.TargetID)
	case LangPython:
		resolved, ok = r.resolvePython(edge.TargetID, edge.SourceID)
	case LangRust:
		resolved, ok = r.resolveRust(edge.TargetID, edge.SourceID)
	default:
		return edge, false
	}

	if !ok {
		return edge, false
	}

	edge.TargetID = resolved
	return edge, true
}

// ResolveAll resolves a slice of edges, dropping unresolvable IMPORTS edges.
// Non-IMPORTS edges pass through unchanged.
func (r *Resolver) ResolveAll(edges []Edge, lang Language) []Edge {
	out := make([]Edge, 0, len(edges))
	for _, e := range edges {
		resolved, ok := r.ResolveEdge(e, lang)
		if ok {
			out = append(out, resolved)
		}
	}
	return out
}

// --- TypeScript resolution ---

var tsExtensions = []string{".ts", ".tsx", ".js", ".jsx", "/index.ts", "/index.tsx", "/index.js"}

func (r *Resolver) resolveTS(importPath, sourceFile string) (string, bool) {
	// Relative imports.
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		sourceDir := filepath.Dir(sourceFile)
		base := filepath.Join(sourceDir, importPath)
		base = filepath.Clean(base)
		return r.probeFile(base, tsExtensions)
	}

	// Workspace package imports.
	return r.resolveTSWorkspace(importPath)
}

func (r *Resolver) resolveTSWorkspace(importPath string) (string, bool) {
	// Try exact match first (e.g. "@test/logger" → mainFile).
	if ws, ok := r.tsWorkspaces[importPath]; ok {
		if ws.mainFile != "" {
			return ws.mainFile, true
		}
		return "", false // workspace has no default export
	}

	// Try splitting into package + subpath.
	// For scoped packages: "@scope/pkg/sub/path" → package="@scope/pkg", subpath="./sub/path"
	// For unscoped: "pkg/sub/path" → package="pkg", subpath="./sub/path"
	var pkgName, subpath string
	if strings.HasPrefix(importPath, "@") {
		// Scoped: find second "/" after the scope.
		afterScope := strings.Index(importPath[1:], "/")
		if afterScope == -1 {
			return "", false // bare @scope (invalid)
		}
		scopeEnd := afterScope + 1 // index of first "/"
		secondSlash := strings.Index(importPath[scopeEnd+1:], "/")
		if secondSlash == -1 {
			return "", false // no subpath, and exact match already failed
		}
		splitAt := scopeEnd + 1 + secondSlash
		pkgName = importPath[:splitAt]
		subpath = "./" + importPath[splitAt+1:]
	} else {
		// Unscoped: "pkg/sub" → package="pkg", subpath="./sub"
		slash := strings.Index(importPath, "/")
		if slash == -1 {
			return "", false // bare package, exact match already failed
		}
		pkgName = importPath[:slash]
		subpath = "./" + importPath[slash+1:]
	}

	ws, ok := r.tsWorkspaces[pkgName]
	if !ok {
		return "", false // external package
	}

	// Check subpath exports.
	if target, ok := ws.subpathExports[subpath]; ok {
		return target, true
	}

	// Fallback: try resolving subpath as a file relative to the workspace dir.
	relPath := subpath[2:] // strip "./"
	base := filepath.Join(ws.dir, relPath)
	return r.probeFile(base, tsExtensions)
}

// --- Go resolution ---

func (r *Resolver) resolveGo(importPath string) (string, bool) {
	if r.goModPath == "" {
		return "", false
	}
	if !strings.HasPrefix(importPath, r.goModPath) {
		return "", false // stdlib or external module
	}

	// Strip module path to get repo-relative directory.
	relDir := strings.TrimPrefix(importPath, r.goModPath)
	relDir = strings.TrimPrefix(relDir, "/")

	// Find the first .go file in that directory.
	files := r.dirIndex[relDir]
	if len(files) == 0 {
		return "", false
	}

	// Sort for determinism, pick first .go file.
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)
	for _, f := range sorted {
		if strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
			return f, true
		}
	}
	return "", false
}

// --- Python resolution ---

func (r *Resolver) resolvePython(importPath, sourceFile string) (string, bool) {
	if !strings.HasPrefix(importPath, ".") {
		return "", false // absolute import (external package)
	}

	// Count leading dots for parent directory traversal.
	dots := 0
	for _, c := range importPath {
		if c == '.' {
			dots++
		} else {
			break
		}
	}

	modulePart := importPath[dots:]

	// Start from source file's directory, go up (dots-1) levels.
	// One dot = same package (current dir), two dots = parent, etc.
	baseDir := filepath.Dir(sourceFile)
	for i := 1; i < dots; i++ {
		baseDir = filepath.Dir(baseDir)
	}

	if modulePart == "" {
		// Bare relative import (just dots) — resolve to __init__.py.
		return r.probeFile(filepath.Join(baseDir, "__init__"), []string{".py"})
	}

	// Replace dots in module name with path separators.
	relPath := strings.ReplaceAll(modulePart, ".", "/")
	base := filepath.Join(baseDir, relPath)

	return r.probeFile(base, []string{".py", "/__init__.py"})
}

// --- Rust resolution ---

func (r *Resolver) resolveRust(importPath, sourceFile string) (string, bool) {
	// Strip use-list braces: "crate::model::{Repository, User}" → "crate::model"
	if idx := strings.Index(importPath, "::{"); idx != -1 {
		importPath = importPath[:idx]
	}

	switch {
	case strings.HasPrefix(importPath, "crate::"):
		modulePath := strings.TrimPrefix(importPath, "crate::")
		relPath := strings.ReplaceAll(modulePath, "::", "/")

		// Rust source is typically under src/. Try both src/ prefixed
		// and relative to repo root.
		candidates := []string{
			filepath.Join("src", relPath),
			relPath,
		}
		// Also check relative to the source file's crate root.
		// If sourceFile is "some_crate/src/service.rs", crate root is "some_crate/src".
		if srcDir := findCrateRoot(sourceFile); srcDir != "" {
			candidates = append(candidates, filepath.Join(srcDir, relPath))
		}

		for _, base := range candidates {
			if resolved, ok := r.probeFile(base, []string{".rs", "/mod.rs"}); ok {
				return resolved, true
			}
		}
		return "", false

	case strings.HasPrefix(importPath, "self::"):
		modulePath := strings.TrimPrefix(importPath, "self::")
		relPath := strings.ReplaceAll(modulePath, "::", "/")
		sourceDir := filepath.Dir(sourceFile)
		base := filepath.Join(sourceDir, relPath)
		return r.probeFile(base, []string{".rs", "/mod.rs"})

	case strings.HasPrefix(importPath, "super::"):
		modulePath := strings.TrimPrefix(importPath, "super::")
		relPath := strings.ReplaceAll(modulePath, "::", "/")
		parentDir := filepath.Dir(filepath.Dir(sourceFile))
		base := filepath.Join(parentDir, relPath)
		return r.probeFile(base, []string{".rs", "/mod.rs"})

	default:
		return "", false // external crate
	}
}

// findCrateRoot walks up from a file path to find the nearest "src" directory,
// which is the conventional Rust crate source root.
func findCrateRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	for dir != "." && dir != "/" && dir != "" {
		if filepath.Base(dir) == "src" {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// --- Shared helpers ---

// probeFile checks if basePath (with any of the given extensions appended)
// exists in the known file set. No filesystem I/O.
func (r *Resolver) probeFile(basePath string, extensions []string) (string, bool) {
	if r.fileSet[basePath] {
		return basePath, true
	}
	for _, ext := range extensions {
		candidate := basePath + ext
		if r.fileSet[candidate] {
			return candidate, true
		}
	}
	return "", false
}

// --- Workspace / module scanning ---

// packageJSON is a minimal representation for reading package.json files.
type packageJSON struct {
	Name       string          `json:"name"`
	Main       string          `json:"main"`
	Workspaces json.RawMessage `json:"workspaces"`
	Exports    json.RawMessage `json:"exports"`
}

func (r *Resolver) scanTSWorkspaces() {
	rootPkg := filepath.Join(r.repoRoot, "package.json")
	data, err := os.ReadFile(rootPkg)
	if err != nil {
		return
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	// Parse workspaces field — can be array of globs or object with "packages" key.
	patterns := parseWorkspacePatterns(pkg.Workspaces)
	if len(patterns) == 0 {
		return
	}

	// Expand glob patterns to find workspace directories.
	for _, pattern := range patterns {
		absPattern := filepath.Join(r.repoRoot, pattern)
		matches, err := filepath.Glob(absPattern)
		if err != nil {
			continue
		}
		for _, dir := range matches {
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				continue
			}
			r.loadWorkspacePackage(dir)
		}
	}
}

func parseWorkspacePatterns(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try as array of strings first: ["packages/*", "apps/*"]
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}

	// Try as object with "packages" key: {"packages": ["packages/*"]}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Packages
	}

	return nil
}

func (r *Resolver) loadWorkspacePackage(absDir string) {
	pkgPath := filepath.Join(absDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Name == "" {
		return
	}

	relDir, err := filepath.Rel(r.repoRoot, absDir)
	if err != nil {
		return
	}

	ws := &tsWorkspace{
		dir:            relDir,
		subpathExports: make(map[string]string),
	}

	// Parse exports field.
	r.parseExports(ws, pkg.Exports)

	// Fallback to "main" if no default export found.
	if ws.mainFile == "" && pkg.Main != "" {
		candidate := filepath.Join(relDir, pkg.Main)
		candidate = filepath.Clean(candidate)
		if r.fileSet[candidate] {
			ws.mainFile = candidate
		} else if resolved, ok := r.probeFile(candidate, tsExtensions); ok {
			ws.mainFile = resolved
		}
	}

	// Last resort: try index.ts / index.js in the package root or src/.
	if ws.mainFile == "" {
		for _, try := range []string{
			filepath.Join(relDir, "src", "index"),
			filepath.Join(relDir, "index"),
		} {
			if resolved, ok := r.probeFile(try, tsExtensions); ok {
				ws.mainFile = resolved
				break
			}
		}
	}

	r.tsWorkspaces[pkg.Name] = ws
}

func (r *Resolver) parseExports(ws *tsWorkspace, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}

	// Try as a simple string: "exports": "./src/index.ts"
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		resolved := filepath.Clean(filepath.Join(ws.dir, str))
		if r.fileSet[resolved] {
			ws.mainFile = resolved
		} else if probed, ok := r.probeFile(resolved, tsExtensions); ok {
			ws.mainFile = probed
		}
		return
	}

	// Try as an object: "exports": {".": "./src/index.ts", "./queries": "./src/queries.ts"}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return
	}

	for key, val := range obj {
		target := resolveExportValue(val)
		if target == "" {
			continue
		}

		resolved := filepath.Clean(filepath.Join(ws.dir, target))
		var finalPath string
		if r.fileSet[resolved] {
			finalPath = resolved
		} else if probed, ok := r.probeFile(resolved, tsExtensions); ok {
			finalPath = probed
		} else {
			continue
		}

		if key == "." {
			ws.mainFile = finalPath
		} else {
			ws.subpathExports[key] = finalPath
		}
	}
}

// resolveExportValue extracts a file path from an export value, which can be
// a string or a conditional object {"import": "...", "require": "...", "default": "..."}.
func resolveExportValue(raw json.RawMessage) string {
	// Try as plain string.
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	// Try as conditional object — prefer "import", then "default", then "require".
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}

	for _, key := range []string{"import", "default", "require"} {
		if v, ok := obj[key]; ok {
			// Recurse: conditional values can themselves be strings or nested objects.
			return resolveExportValue(v)
		}
	}
	return ""
}

func (r *Resolver) scanGoMod() {
	modPath := filepath.Join(r.repoRoot, "go.mod")
	f, err := os.Open(modPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			r.goModPath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			return
		}
	}
}
