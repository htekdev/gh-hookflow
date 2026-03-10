package e2e

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// assertionCallNames contains function/method names that count as test assertions.
var assertionCallNames = map[string]bool{
	"Error":    true,
	"Errorf":   true,
	"Fatal":    true,
	"Fatalf":   true,
	"FailNow":  true,
	"Fail":     true,
	"Log":      false, // explicitly NOT an assertion
	"Logf":     false,
	"Helper":   false,
	"Cleanup":  false,
	"Run":      false, // subtests counted separately
	"Parallel": false,
	"TempDir":  false,
	"Setenv":   false,
	"Skip":     false,
	"Skipf":    false,
}

// customAssertionFuncs are project-specific helper functions that contain assertions.
var customAssertionFuncs = map[string]bool{
	"assertDeny":  true,
	"assertAllow": true,
}

// badTestNamePattern matches non-descriptive test names like Test1, TestA, TestFoo1.
var badTestNamePattern = regexp.MustCompile(`^Test[A-Z]?[0-9]*$`)

// TestQualityAllTestsHaveAssertions ensures every Test function in the E2E suite
// contains at least one real assertion (t.Error, t.Fatal, assertDeny, etc.).
// Tests that only log output or have empty bodies are flagged.
func TestQualityAllTestsHaveAssertions(t *testing.T) {
	testFiles := findTestFiles(t)
	fset := token.NewFileSet()

	for _, file := range testFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}

		f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", file, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if !isTestFunction(fn) {
				continue
			}

			// Skip this meta-test (test_quality_test.go tests)
			if strings.HasPrefix(fn.Name.Name, "TestQuality") {
				continue
			}

			relPath := relativeToE2E(file)
			pos := fset.Position(fn.Pos())

			if fn.Body == nil || len(fn.Body.List) == 0 {
				t.Errorf("%s:%d: %s has an empty body — every test must have assertions",
					relPath, pos.Line, fn.Name.Name)
				continue
			}

			if !hasAssertion(fn.Body) && !hasSubtests(fn.Body) {
				t.Errorf("%s:%d: %s has no assertions (t.Error, t.Fatal, assertDeny, assertAllow) — tests must verify behavior",
					relPath, pos.Line, fn.Name.Name)
			}
		}
	}
}

// TestQualityNoUnjustifiedSkips ensures t.Skip is only used with platform/environment checks.
func TestQualityNoUnjustifiedSkips(t *testing.T) {
	testFiles := findTestFiles(t)
	fset := token.NewFileSet()

	for _, file := range testFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}

		f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", file, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isTestFunction(fn) || fn.Body == nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "TestQuality") {
				continue
			}

			checkSkipsInBody(t, fset, fn.Body, fn.Name.Name, file)
		}
	}
}

// TestQualityDescriptiveNames ensures test functions have descriptive names.
func TestQualityDescriptiveNames(t *testing.T) {
	testFiles := findTestFiles(t)
	fset := token.NewFileSet()

	for _, file := range testFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}

		f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", file, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isTestFunction(fn) {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "TestQuality") {
				continue
			}

			relPath := relativeToE2E(file)
			pos := fset.Position(fn.Pos())

			if badTestNamePattern.MatchString(fn.Name.Name) {
				t.Errorf("%s:%d: %s has a non-descriptive name — test names must describe behavior",
					relPath, pos.Line, fn.Name.Name)
			}
		}
	}
}

// TestQualityNoPlaceholderComments ensures no TODO/FIXME/HACK comments inside test functions.
func TestQualityNoPlaceholderComments(t *testing.T) {
	testFiles := findTestFiles(t)
	fset := token.NewFileSet()

	for _, file := range testFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", file, err)
		}

		f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", file, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isTestFunction(fn) || fn.Body == nil {
				continue
			}
			if strings.HasPrefix(fn.Name.Name, "TestQuality") {
				continue
			}

			relPath := relativeToE2E(file)

			// Check comments within the function body
			for _, cg := range f.Comments {
				commentPos := fset.Position(cg.Pos())
				fnStart := fset.Position(fn.Body.Lbrace)
				fnEnd := fset.Position(fn.Body.Rbrace)

				if commentPos.Line >= fnStart.Line && commentPos.Line <= fnEnd.Line {
					for _, c := range cg.List {
						text := strings.TrimSpace(c.Text)
						if strings.Contains(text, "TODO") || strings.Contains(text, "FIXME") || strings.Contains(text, "HACK") {
							cPos := fset.Position(c.Pos())
							t.Errorf("%s:%d: placeholder comment %q in %s — use issues for follow-up work, not code comments",
								relPath, cPos.Line, text, fn.Name.Name)
						}
					}
				}
			}
		}
	}
}

// --- Helpers ---

func findTestFiles(t *testing.T) []string {
	t.Helper()

	e2eDir := "."
	if _, err := os.Stat("tests/e2e"); err == nil {
		e2eDir = "tests/e2e"
	}

	entries, err := os.ReadDir(e2eDir)
	if err != nil {
		t.Fatalf("Failed to read E2E directory %s: %v", e2eDir, err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, "_test.go") {
			// Exclude helper files that don't contain test functions
			if name == "e2e_test.go" || name == "helpers_test.go" {
				continue
			}
			files = append(files, filepath.Join(e2eDir, name))
		}
	}

	if len(files) == 0 {
		t.Fatal("No test files found in E2E directory")
	}

	return files
}

func isTestFunction(fn *ast.FuncDecl) bool {
	if fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}
	// Must have exactly one parameter of type *testing.T
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	starExpr, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "testing" && selExpr.Sel.Name == "T"
}

func hasAssertion(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for t.Error, t.Errorf, t.Fatal, t.Fatalf
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if assertionCallNames[sel.Sel.Name] {
				found = true
				return false
			}
		}

		// Check for custom assertion functions (assertDeny, assertAllow)
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if customAssertionFuncs[ident.Name] {
				found = true
				return false
			}
		}

		return true
	})
	return found
}

func hasSubtests(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Run" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func checkSkipsInBody(t *testing.T, fset *token.FileSet, body *ast.BlockStmt, funcName, file string) {
	t.Helper()
	relPath := relativeToE2E(file)

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if sel.Sel.Name != "Skip" && sel.Sel.Name != "Skipf" {
			return true
		}

		// Found a t.Skip call — check if the enclosing if statement
		// has a platform/environment justification
		skipPos := fset.Position(call.Pos())

		if !isSkipJustified(body, call, fset) {
			t.Errorf("%s:%d: unjustified %s.%s() in %s — t.Skip is only allowed with platform/environment checks (runtime.GOOS, os.Getenv, exec.LookPath)",
				relPath, skipPos.Line, exprToString(sel.X), sel.Sel.Name, funcName)
		}

		return true
	})
}

func isSkipJustified(body *ast.BlockStmt, skipCall *ast.CallExpr, fset *token.FileSet) bool {
	skipLine := fset.Position(skipCall.Pos()).Line

	// Walk the body looking for if statements that contain this skip
	justified := false
	ast.Inspect(body, func(n ast.Node) bool {
		if justified {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}

		// Check if this if statement contains the skip call
		ifStart := fset.Position(ifStmt.Pos()).Line
		ifEnd := fset.Position(ifStmt.End()).Line
		if skipLine < ifStart || skipLine > ifEnd {
			return true
		}

		// Check if the condition references justified patterns
		condStr := nodeToString(ifStmt.Cond)
		justifiedPatterns := []string{
			"runtime.GOOS",
			"GOOS",
			"exec.LookPath",
			"os.Getenv",
			"testing.Short",
		}
		for _, pattern := range justifiedPatterns {
			if strings.Contains(condStr, pattern) {
				justified = true
				return false
			}
		}

		return true
	})
	return justified
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	default:
		return "?"
	}
}

func nodeToString(node ast.Node) string {
	var sb strings.Builder
	ast.Inspect(node, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.Ident:
			sb.WriteString(e.Name)
			sb.WriteString(" ")
		case *ast.SelectorExpr:
			sb.WriteString(exprToString(e))
			sb.WriteString(" ")
			return false
		}
		return true
	})
	return sb.String()
}

func relativeToE2E(file string) string {
	if idx := strings.Index(file, "tests/e2e/"); idx >= 0 {
		return file[idx:]
	}
	if idx := strings.Index(file, `tests\e2e\`); idx >= 0 {
		return strings.ReplaceAll(file[idx:], `\`, "/")
	}
	return filepath.Base(file)
}
