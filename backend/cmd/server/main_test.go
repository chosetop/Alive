package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestRunWiresWorldServiceIntoEntryService(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var worldServicePos token.Pos
	var entryServiceCall *ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name != "worldService" {
				continue
			}
			if i < len(assign.Rhs) {
				worldServicePos = assign.Rhs[i].Pos()
			}
		}
		return true
	})
	if !worldServicePos.IsValid() {
		t.Fatal("run must create worldService before entryService")
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if entryServiceCall != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewService" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "entry" {
			return true
		}
		entryServiceCall = call
		return false
	})
	if entryServiceCall == nil {
		t.Fatal("run must create entryService with entry.NewService")
	}
	if worldServicePos > entryServiceCall.Pos() {
		t.Fatal("worldService must be created before entryService so it can be injected")
	}

	for _, arg := range entryServiceCall.Args {
		call, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WithWorldService" {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "entry" {
			continue
		}
		if len(call.Args) != 1 {
			t.Fatalf("entry.WithWorldService arg count = %d, want 1", len(call.Args))
		}
		argIdent, ok := call.Args[0].(*ast.Ident)
		if ok && argIdent.Name == "worldService" {
			return
		}
		t.Fatalf("entry.WithWorldService arg = %#v, want worldService", call.Args[0])
	}

	t.Fatal("entry.NewService must include entry.WithWorldService(worldService)")
}
