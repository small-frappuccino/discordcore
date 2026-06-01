package storage

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestStorageQueriesUsePositionalPlaceholders fails when any SQL string in the
// storage package uses a MySQL/SQLite-style "?" bind placeholder. The package
// talks to PostgreSQL through the pgx stdlib driver, which accepts only
// positional "$1, $2" placeholders; a "?" is forwarded to the server verbatim
// and fails at execution time with SQLSTATE 42601 (syntax_error) — for example
// `syntax error at or near "AND"` when the token after the "?" is a keyword.
//
// The check is a static source scan with no build tag, so it runs under the
// default `go test ./...`. The behavioral store tests are gated behind
// //go:build integration and a configured DATABASE_URL, so they are skipped
// wherever no live database is reachable — which is how "?" placeholders reached
// production unnoticed. This guard closes that gap.
//
// Every "?" inside an SQL string is treated as a bind placeholder. A future
// query needing the PostgreSQL JSONB key-existence operator should use the
// jsonb_exists() function form instead of "?", so this guard stays reliable.
func TestStorageQueriesUsePositionalPlaceholders(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob storage package go files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no go files found in storage package working directory")
	}

	looksLikeSQL := func(literal string) bool {
		upper := strings.ToUpper(literal)
		for _, keyword := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WHERE", "VALUES"} {
			if strings.Contains(upper, keyword) {
				return true
			}
		}
		return false
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !looksLikeSQL(lit.Value) {
				return true
			}
			start := fset.Position(lit.Pos())
			for offset, line := range strings.Split(lit.Value, "\n") {
				if strings.Contains(line, "?") {
					t.Errorf("%s:%d: SQL uses a '?' bind placeholder; the pgx/PostgreSQL driver requires positional '$N' placeholders", start.Filename, start.Line+offset)
				}
			}
			return true
		})
	}
}
