// Command nocomments enforces the repository comment policy: comments are only
// allowed as doc comments on exported API and as compiler directives.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var directivePrefixes = []string{
	"//go:",
	"//line ",
	"//export ",
	"//nolint",
}

var skippedDirs = map[string]bool{
	".git":     true,
	"vendor":   true,
	"testdata": true,
}

var markerPattern = regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)\b`)

func main() {
	targets := os.Args[1:]
	if len(targets) == 0 {
		targets = []string{"."}
	}

	var violations []string
	for _, target := range targets {
		found, err := scan(strings.TrimSuffix(target, "/..."))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		violations = append(violations, found...)
	}

	for _, v := range violations {
		fmt.Fprintln(os.Stderr, v)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "%d forbidden comment(s) found: only doc comments on exported API are allowed\n", len(violations))
		os.Exit(1)
	}
}

func scan(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		found, err := inspect(path, nil)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	return violations, err
}

func inspect(path string, src any) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}

	docs := exportedDocs(file)
	if strings.HasSuffix(path, "_test.go") {
		allowExampleOutputs(file, docs)
	}

	var violations []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if isDirective(comment.Text) {
				continue
			}
			position := fset.Position(comment.Pos())
			switch {
			case markerPattern.MatchString(comment.Text):
				violations = append(violations, fmt.Sprintf("%s: task marker: %s", position, firstLine(comment.Text)))
			case !docs[group]:
				violations = append(violations, fmt.Sprintf("%s: %s", position, firstLine(comment.Text)))
			}
		}
	}
	return violations, nil
}

func exportedDocs(file *ast.File) map[*ast.CommentGroup]bool {
	docs := map[*ast.CommentGroup]bool{}
	allow := func(group *ast.CommentGroup) {
		if group != nil {
			docs[group] = true
		}
	}

	allow(file.Doc)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if isExportedFunc(d) {
				allow(d.Doc)
			}
		case *ast.GenDecl:
			exportedGroup := false
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						exportedGroup = true
						allow(s.Doc)
						allowExportedFields(s.Type, allow)
					}
				case *ast.ValueSpec:
					if anyExported(s.Names) {
						exportedGroup = true
						allow(s.Doc)
					}
				}
			}
			if exportedGroup {
				allow(d.Doc)
			}
		}
	}
	return docs
}

func allowExampleOutputs(file *ast.File, docs map[*ast.CommentGroup]bool) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Example") {
			continue
		}
		for _, group := range file.Comments {
			if group.Pos() < fn.Body.Lbrace || group.End() > fn.Body.Rbrace {
				continue
			}
			first := strings.TrimSpace(strings.TrimPrefix(group.List[0].Text, "//"))
			if first == "Output:" || first == "Unordered output:" {
				docs[group] = true
			}
		}
	}
}

func isExportedFunc(fn *ast.FuncDecl) bool {
	if !fn.Name.IsExported() {
		return false
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return true
	}
	return receiverTypeName(fn.Recv.List[0].Type).IsExported()
}

func receiverTypeName(expr ast.Expr) *ast.Ident {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t
	}
	return ast.NewIdent("_")
}

func allowExportedFields(expr ast.Expr, allow func(*ast.CommentGroup)) {
	var fields *ast.FieldList
	switch t := expr.(type) {
	case *ast.StructType:
		fields = t.Fields
	case *ast.InterfaceType:
		fields = t.Methods
	default:
		return
	}
	for _, field := range fields.List {
		if len(field.Names) == 0 || anyExported(field.Names) {
			allow(field.Doc)
		}
	}
}

func anyExported(names []*ast.Ident) bool {
	for _, name := range names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func isDirective(text string) bool {
	for _, prefix := range directivePrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
