package template

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"
)

// formatGo drops the imports src does not use and formats it with go/format,
// so a template can import every package it might need.
func formatGo(src []byte) ([]byte, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	used := make(map[string]bool)

	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}

		return true
	})

	var kept []string

	for _, imp := range file.Imports {
		if !isUnusedImport(imp, used) {
			line := imp.Path.Value
			if imp.Name != nil {
				line = imp.Name.Name + " " + line
			}

			kept = append(kept, line)
		}
	}

	// Print the file without its import declarations, then put back a single
	// block of the kept ones: removing specs in place leaves blank lines that
	// gofmt reads as import groups.
	decls := file.Decls[:0]

	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); !ok || gen.Tok != token.IMPORT {
			decls = append(decls, decl)
		}
	}

	file.Decls = decls

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, err
	}

	out := buf.String()

	if len(kept) > 0 {
		pkg := 0
		if !strings.HasPrefix(out, "package ") {
			pkg = strings.Index(out, "\npackage ") + 1
		}

		end := pkg + strings.IndexByte(out[pkg:], '\n') + 1
		out = out[:end] + "\nimport (\n" + strings.Join(kept, "\n") + "\n)\n" + out[end:]
	}

	return format.Source([]byte(out))
}

// isUnusedImport reports whether imp is a named or plain import no selector
// in the file refers to; blank and dot imports always stay.
func isUnusedImport(imp *ast.ImportSpec, used map[string]bool) bool {
	importPath, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		return false
	}

	name := path.Base(importPath)
	if imp.Name != nil {
		name = imp.Name.Name
	}

	return name != "_" && name != "." && !used[name]
}
