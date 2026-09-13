//go:build integration

package excelexchange

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

func TestE7P2PreviewMatrixIDsCompleteAndUnique(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	found := map[int]string{}
	re := regexp.MustCompile(`^TestE7P2INT(\d{2})`)
	fset := token.NewFileSet()
	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			m := re.FindStringSubmatch(fn.Name.Name)
			if len(m) != 2 {
				continue
			}
			id, convErr := strconv.Atoi(m[1])
			if convErr != nil {
				return convErr
			}
			if id < 21 || id > 42 {
				continue
			}
			if prev, exists := found[id]; exists {
				return fmt.Errorf("duplicate test id INT-%02d: %s and %s", id, prev, fn.Name.Name)
			}
			found[id] = fn.Name.Name
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan tests: %v", err)
	}
	for id := 21; id <= 42; id++ {
		if _, ok := found[id]; !ok {
			t.Fatalf("missing required test E7P2-INT-%02d", id)
		}
	}
}
