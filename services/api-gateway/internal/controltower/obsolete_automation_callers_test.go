package controltower

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestIssue334ObsoleteAutomationFunctionsHaveNoCallers guards the three unused
// Control Tower helpers. A later deletion may remove the declarations. Until
// then, any new call or value reference fails this test.
func TestIssue334ObsoleteAutomationFunctionsHaveNoCallers(t *testing.T) {
	names := map[string]struct{}{
		"evaluateAndPersistRisks":       {},
		"triggerExceptionAutomation":    {},
		"triggerExceptionSLAAutomation": {},
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	var uses []string
	record := func(ident *ast.Ident) {
		if ident == nil {
			return
		}
		if _, obsolete := names[ident.Name]; obsolete {
			uses = append(uses, fset.Position(ident.Pos()).String()+": "+ident.Name)
		}
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				decl, ok := n.(*ast.FuncDecl)
				if !ok || decl.Name == nil {
					ident, ok := n.(*ast.Ident)
					if ok {
						record(ident)
					}
					return true
				}
				if _, obsolete := names[decl.Name.Name]; !obsolete {
					return true
				}
				if decl.Body != nil {
					ast.Inspect(decl.Body, func(body ast.Node) bool {
						ident, ok := body.(*ast.Ident)
						if ok {
							record(ident)
						}
						return true
					})
				}
				return false
			})
		}
	}
	if len(uses) != 0 {
		t.Fatalf("obsolete automation functions are referenced:\n%s", strings.Join(uses, "\n"))
	}
}
