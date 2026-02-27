//go:build cgo

package graph

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	kuzu "github.com/kuzudb/go-kuzu"
)

// KuzuStore implements the Store interface using KuzuDB as the graph backend.
// It requires CGO because the go-kuzu driver wraps KuzuDB's C library.
type KuzuStore struct {
	db   *kuzu.Database
	conn *kuzu.Connection
}

// Compile-time check that KuzuStore satisfies Store.
var _ Store = (*KuzuStore)(nil)

// NewKuzuStore creates a KuzuStore backed by an in-memory KuzuDB instance.
func NewKuzuStore() (*KuzuStore, error) {
	cfg := kuzu.DefaultSystemConfig()
	db, err := kuzu.OpenDatabase(":memory:", cfg)
	if err != nil {
		return nil, fmt.Errorf("kuzu: open database: %w", err)
	}
	conn, err := kuzu.OpenConnection(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("kuzu: open connection: %w", err)
	}
	return &KuzuStore{db: db, conn: conn}, nil
}

// NewKuzuFileStore creates a KuzuStore backed by a file-based KuzuDB at the
// given directory path. KuzuDB creates the directory itself for new databases.
// For existing databases, the directory must contain valid KuzuDB files.
// This enables persistent graph indexes that survive across sessions.
func NewKuzuFileStore(dbPath string) (*KuzuStore, error) {
	// Ensure parent directory exists (KuzuDB creates the leaf directory).
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("kuzu: create parent directory: %w", err)
	}
	cfg := kuzu.DefaultSystemConfig()
	db, err := kuzu.OpenDatabase(dbPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("kuzu: open file database: %w", err)
	}
	conn, err := kuzu.OpenConnection(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("kuzu: open connection: %w", err)
	}
	return &KuzuStore{db: db, conn: conn}, nil
}

// Close releases the KuzuDB connection and database.
func (s *KuzuStore) Close() error {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	return nil
}

// ---------- Schema setup ----------

// ddlStatements defines the Cypher DDL executed by InitSchema.
// Order matters: node tables must precede relationship tables.
var ddlStatements = []string{
	`CREATE NODE TABLE IF NOT EXISTS File(
		path STRING,
		language STRING,
		loc INT64,
		PRIMARY KEY(path)
	)`,
	`CREATE NODE TABLE IF NOT EXISTS Symbol(
		id STRING,
		name STRING,
		kind STRING,
		exported BOOLEAN,
		file_path STRING,
		start_line INT64,
		end_line INT64,
		PRIMARY KEY(id)
	)`,
	`CREATE NODE TABLE IF NOT EXISTS Cluster(
		name STRING,
		cohesion_score DOUBLE,
		PRIMARY KEY(name)
	)`,
	`CREATE REL TABLE IF NOT EXISTS DEFINES(FROM File TO Symbol)`,
	`CREATE REL TABLE IF NOT EXISTS IMPORTS(FROM File TO File)`,
	`CREATE REL TABLE IF NOT EXISTS CALLS(FROM Symbol TO Symbol)`,
	`CREATE REL TABLE IF NOT EXISTS INHERITS_FROM(FROM Symbol TO Symbol)`,
	`CREATE REL TABLE IF NOT EXISTS IMPLEMENTS(FROM Symbol TO Symbol)`,
	`CREATE REL TABLE IF NOT EXISTS BELONGS_TO(FROM File TO Cluster)`,
}

// InitSchema creates all node and relationship tables if they do not exist.
func (s *KuzuStore) InitSchema(_ context.Context) error {
	for _, stmt := range ddlStatements {
		res, err := s.conn.Query(stmt)
		if err != nil {
			return fmt.Errorf("kuzu: init schema: %w", err)
		}
		res.Close()
	}
	return nil
}

// ---------- Write operations ----------

// AddFile inserts a File node.
func (s *KuzuStore) AddFile(_ context.Context, node FileNode) error {
	return s.exec(
		"CREATE (f:File {path: $path, language: $lang, loc: $loc})",
		map[string]any{
			"path": node.Path,
			"lang": string(node.Language),
			"loc":  int64(node.LOC),
		},
	)
}

// AddSymbol inserts a Symbol node.
func (s *KuzuStore) AddSymbol(_ context.Context, node SymbolNode) error {
	return s.exec(
		`CREATE (s:Symbol {
			id: $id,
			name: $name,
			kind: $kind,
			exported: $exported,
			file_path: $fp,
			start_line: $sl,
			end_line: $el
		})`,
		map[string]any{
			"id":       symbolID(node.FilePath, node.Name),
			"name":     node.Name,
			"kind":     string(node.Kind),
			"exported": node.Exported,
			"fp":       node.FilePath,
			"sl":       int64(node.StartLine),
			"el":       int64(node.EndLine),
		},
	)
}

// AddCluster inserts a Cluster node.
func (s *KuzuStore) AddCluster(_ context.Context, node ClusterNode) error {
	return s.exec(
		"CREATE (c:Cluster {name: $name, cohesion_score: $score})",
		map[string]any{
			"name":  node.Name,
			"score": node.CohesionScore,
		},
	)
}

// AddEdge inserts a relationship edge between two nodes.
// The Cypher statement is chosen based on the EdgeKind.
func (s *KuzuStore) AddEdge(_ context.Context, edge Edge) error {
	cypher, err := edgeCypher(edge.Kind)
	if err != nil {
		return err
	}
	return s.exec(cypher, map[string]any{
		"src": edge.SourceID,
		"dst": edge.TargetID,
	})
}

// edgeCypher returns the MATCH-CREATE Cypher for the given edge kind.
func edgeCypher(kind EdgeKind) (string, error) {
	switch kind {
	case EdgeKindDefines:
		return `MATCH (a:File {path: $src}), (b:Symbol {id: $dst})
				CREATE (a)-[:DEFINES]->(b)`, nil
	case EdgeKindImports:
		return `MATCH (a:File {path: $src}), (b:File {path: $dst})
				CREATE (a)-[:IMPORTS]->(b)`, nil
	case EdgeKindCalls:
		return `MATCH (a:Symbol {id: $src}), (b:Symbol {id: $dst})
				CREATE (a)-[:CALLS]->(b)`, nil
	case EdgeKindInherits:
		return `MATCH (a:Symbol {id: $src}), (b:Symbol {id: $dst})
				CREATE (a)-[:INHERITS_FROM]->(b)`, nil
	case EdgeKindImplements:
		return `MATCH (a:Symbol {id: $src}), (b:Symbol {id: $dst})
				CREATE (a)-[:IMPLEMENTS]->(b)`, nil
	case EdgeKindBelongs:
		return `MATCH (a:File {path: $src}), (b:Cluster {name: $dst})
				CREATE (a)-[:BELONGS_TO]->(b)`, nil
	default:
		return "", fmt.Errorf("kuzu: unsupported edge kind: %s", kind)
	}
}

// ---------- Read operations ----------

// GetFile retrieves a single File node by path, or returns nil if not found.
func (s *KuzuStore) GetFile(_ context.Context, path string) (*FileNode, error) {
	rows, err := s.query(
		"MATCH (f:File {path: $path}) RETURN f.path, f.language, f.loc",
		map[string]any{"path": path},
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	r := rows[0]
	return &FileNode{
		Path:     toString(r[0]),
		Language: Language(toString(r[1])),
		LOC:      toInt(r[2]),
	}, nil
}

// GetSymbol retrieves a single Symbol node by file path and name, or nil if not found.
func (s *KuzuStore) GetSymbol(_ context.Context, filePath, name string) (*SymbolNode, error) {
	rows, err := s.query(
		`MATCH (s:Symbol {id: $id})
		 RETURN s.name, s.kind, s.exported, s.file_path, s.start_line, s.end_line`,
		map[string]any{"id": symbolID(filePath, name)},
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rowToSymbol(rows[0]), nil
}

// QuerySymbols returns symbols whose name contains the query string.
func (s *KuzuStore) QuerySymbols(_ context.Context, queryStr string, limit int) ([]SymbolNode, error) {
	rows, err := s.query(
		`MATCH (s:Symbol) WHERE s.name CONTAINS $q
		 RETURN s.name, s.kind, s.exported, s.file_path, s.start_line, s.end_line
		 LIMIT $lim`,
		map[string]any{
			"q":   queryStr,
			"lim": int64(limit),
		},
	)
	if err != nil {
		return nil, err
	}
	out := make([]SymbolNode, 0, len(rows))
	for _, r := range rows {
		out = append(out, *rowToSymbol(r))
	}
	return out, nil
}

// ---------- Graph traversal ----------

// GetDependencies performs a BFS over IMPORTS edges starting from the given
// file path. It returns one DependencyChain per reachable file.
func (s *KuzuStore) GetDependencies(_ context.Context, nodeID string, dir Direction, maxDepth int) ([]DependencyChain, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}

	// BFS state.
	type bfsEntry struct {
		path  []string
		depth int
	}
	visited := map[string]bool{nodeID: true}
	queue := []bfsEntry{{path: []string{nodeID}, depth: 0}}
	var chains []DependencyChain

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		tip := cur.path[len(cur.path)-1]
		neighbors, err := s.fileNeighbors(tip, dir)
		if err != nil {
			return nil, err
		}
		for _, nb := range neighbors {
			if visited[nb] {
				continue
			}
			visited[nb] = true
			newPath := make([]string, len(cur.path)+1)
			copy(newPath, cur.path)
			newPath[len(cur.path)] = nb
			chains = append(chains, DependencyChain{
				Nodes: newPath,
				Depth: cur.depth + 1,
			})
			queue = append(queue, bfsEntry{path: newPath, depth: cur.depth + 1})
		}
	}
	return chains, nil
}

// fileNeighbors returns immediate file neighbors along IMPORTS edges.
func (s *KuzuStore) fileNeighbors(path string, dir Direction) ([]string, error) {
	var cypher string
	switch dir {
	case DirectionDownstream:
		cypher = "MATCH (a:File {path: $path})-[:IMPORTS]->(b:File) RETURN b.path"
	case DirectionUpstream:
		cypher = "MATCH (a:File)-[:IMPORTS]->(b:File {path: $path}) RETURN a.path"
	default:
		return nil, fmt.Errorf("kuzu: unknown direction: %s", dir)
	}
	rows, err := s.query(cypher, map[string]any{"path": path})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, toString(r[0]))
	}
	return out, nil
}

// AssessImpact computes the blast radius of the given set of changed files.
// It walks IMPORTS edges downstream to find direct and transitive dependents,
// then computes a risk score from the fan-out ratio.
func (s *KuzuStore) AssessImpact(ctx context.Context, changedFiles []string) (*ImpactResult, error) {
	totalFiles, err := s.countTable("File")
	if err != nil {
		return nil, err
	}

	directSet := map[string]bool{}
	transitiveSet := map[string]bool{}

	for _, f := range changedFiles {
		chains, err := s.GetDependencies(ctx, f, DirectionDownstream, 1)
		if err != nil {
			return nil, err
		}
		for _, c := range chains {
			last := c.Nodes[len(c.Nodes)-1]
			directSet[last] = true
		}

		allChains, err := s.GetDependencies(ctx, f, DirectionDownstream, 10)
		if err != nil {
			return nil, err
		}
		for _, c := range allChains {
			last := c.Nodes[len(c.Nodes)-1]
			transitiveSet[last] = true
		}
	}

	// Remove changed files themselves from result sets.
	changedMap := map[string]bool{}
	for _, f := range changedFiles {
		changedMap[f] = true
	}
	direct := filterKeys(directSet, changedMap)
	transitive := filterKeys(transitiveSet, changedMap)

	risk := 0.0
	if totalFiles > 0 {
		risk = math.Min(1.0, float64(len(transitive))/float64(totalFiles))
	}

	return &ImpactResult{
		DirectlyAffected:     direct,
		TransitivelyAffected: transitive,
		RiskScore:            risk,
	}, nil
}

// GetClusters returns all Cluster nodes.
func (s *KuzuStore) GetClusters(_ context.Context) ([]ClusterNode, error) {
	rows, err := s.query(
		"MATCH (c:Cluster) RETURN c.name, c.cohesion_score",
		nil,
	)
	if err != nil {
		return nil, err
	}
	out := make([]ClusterNode, 0, len(rows))
	for _, r := range rows {
		name := toString(r[0])
		score := toFloat64(r[1])

		// Fetch cluster members via BELONGS_TO edges.
		memberRows, err := s.query(
			"MATCH (f:File)-[:BELONGS_TO]->(c:Cluster {name: $name}) RETURN f.path",
			map[string]any{"name": name},
		)
		if err != nil {
			return nil, err
		}
		members := make([]string, 0, len(memberRows))
		for _, mr := range memberRows {
			members = append(members, toString(mr[0]))
		}

		out = append(out, ClusterNode{
			Name:          name,
			CohesionScore: score,
			Members:       members,
		})
	}
	return out, nil
}

// ---------- Edge enumeration ----------

// GetAllEdges returns all edges across all relationship tables.
func (s *KuzuStore) GetAllEdges(_ context.Context) ([]Edge, error) {
	type relQuery struct {
		cypher string
		kind   EdgeKind
	}

	queries := []relQuery{
		{"MATCH (a:File)-[:DEFINES]->(b:Symbol) RETURN a.path, b.id", EdgeKindDefines},
		{"MATCH (a:File)-[:IMPORTS]->(b:File) RETURN a.path, b.path", EdgeKindImports},
		{"MATCH (a:Symbol)-[:CALLS]->(b:Symbol) RETURN a.id, b.id", EdgeKindCalls},
		{"MATCH (a:Symbol)-[:INHERITS_FROM]->(b:Symbol) RETURN a.id, b.id", EdgeKindInherits},
		{"MATCH (a:Symbol)-[:IMPLEMENTS]->(b:Symbol) RETURN a.id, b.id", EdgeKindImplements},
		{"MATCH (a:File)-[:BELONGS_TO]->(b:Cluster) RETURN a.path, b.name", EdgeKindBelongs},
	}

	var edges []Edge
	for _, q := range queries {
		rows, err := s.query(q.cypher, nil)
		if err != nil {
			// Table may not exist yet; skip.
			continue
		}
		for _, r := range rows {
			edges = append(edges, Edge{
				SourceID: toString(r[0]),
				TargetID: toString(r[1]),
				Kind:     q.kind,
			})
		}
	}
	return edges, nil
}

// ---------- Stats ----------

// Stats returns counts of all node and edge tables.
func (s *KuzuStore) Stats(_ context.Context) (*GraphStats, error) {
	files, err := s.countTable("File")
	if err != nil {
		return nil, err
	}
	symbols, err := s.countTable("Symbol")
	if err != nil {
		return nil, err
	}
	clusters, err := s.countTable("Cluster")
	if err != nil {
		return nil, err
	}
	edges, err := s.countEdges()
	if err != nil {
		return nil, err
	}
	return &GraphStats{
		FileCount:    files,
		SymbolCount:  symbols,
		ClusterCount: clusters,
		EdgeCount:    edges,
	}, nil
}

// ---------- Internal helpers ----------

// exec runs a parameterized Cypher statement that produces no result rows.
func (s *KuzuStore) exec(cypher string, params map[string]any) error {
	stmt, err := s.conn.Prepare(cypher)
	if err != nil {
		return fmt.Errorf("kuzu: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := s.conn.Execute(stmt, params)
	if err != nil {
		return fmt.Errorf("kuzu: execute: %w", err)
	}
	res.Close()
	return nil
}

// query runs a parameterized Cypher statement and collects all result rows.
// Each row is a []any slice with values in column order.
func (s *KuzuStore) query(cypher string, params map[string]any) ([][]any, error) {
	var res *kuzu.QueryResult
	var err error

	if len(params) == 0 {
		res, err = s.conn.Query(cypher)
	} else {
		var stmt *kuzu.PreparedStatement
		stmt, err = s.conn.Prepare(cypher)
		if err != nil {
			return nil, fmt.Errorf("kuzu: prepare: %w", err)
		}
		defer stmt.Close()
		res, err = s.conn.Execute(stmt, params)
	}
	if err != nil {
		return nil, fmt.Errorf("kuzu: query: %w", err)
	}
	defer res.Close()

	var rows [][]any
	for res.HasNext() {
		tuple, err := res.Next()
		if err != nil {
			return nil, fmt.Errorf("kuzu: next: %w", err)
		}
		vals, err := tuple.GetAsSlice()
		if err != nil {
			return nil, fmt.Errorf("kuzu: row values: %w", err)
		}
		rows = append(rows, vals)
	}
	return rows, nil
}

// countTable returns the number of rows in a node table.
func (s *KuzuStore) countTable(table string) (int, error) {
	// Table name is a fixed internal constant, not user input.
	cypher := fmt.Sprintf("MATCH (n:%s) RETURN count(n)", table)
	rows, err := s.query(cypher, nil)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		return 0, nil
	}
	return toInt(rows[0][0]), nil
}

// countEdges returns the total number of edges across all relationship tables.
func (s *KuzuStore) countEdges() (int, error) {
	tables := []string{"DEFINES", "IMPORTS", "CALLS", "INHERITS_FROM", "IMPLEMENTS", "BELONGS_TO"}
	total := 0
	for _, t := range tables {
		cypher := fmt.Sprintf("MATCH ()-[r:%s]->() RETURN count(r)", t)
		rows, err := s.query(cypher, nil)
		if err != nil {
			// Table may not exist yet; treat as zero.
			continue
		}
		if len(rows) > 0 && len(rows[0]) > 0 {
			total += toInt(rows[0][0])
		}
	}
	return total, nil
}

// symbolID produces a deterministic identifier for a symbol: "filePath:name".
func symbolID(filePath, name string) string {
	return filePath + ":" + name
}

// rowToSymbol converts a 6-column result row into a SymbolNode.
// Column order: name, kind, exported, file_path, start_line, end_line.
func rowToSymbol(r []any) *SymbolNode {
	return &SymbolNode{
		Name:      toString(r[0]),
		Kind:      SymbolKind(toString(r[1])),
		Exported:  toBool(r[2]),
		FilePath:  toString(r[3]),
		StartLine: toInt(r[4]),
		EndLine:   toInt(r[5]),
	}
}

// filterKeys returns keys from set that are not in exclude, as a sorted slice.
func filterKeys(set, exclude map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		if !exclude[k] {
			out = append(out, k)
		}
	}
	return out
}

// ---------- Type coercion helpers ----------
// KuzuDB returns typed Go values (int64, float64, bool, string).
// These helpers safely coerce any -> concrete type.

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
