package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Package-level vars set by TestMain and used by all tests.
var (
	binaryPath     string // path to coverage-instrumented hookflow binary
	globalCoverDir string // root dir for per-test coverage data
	repoRoot       string // repository root (contains go.mod)
)

func TestMain(m *testing.M) {
	// 1. Find the repository root by walking up from this test file
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot = root

	// 2. Create a temp directory for coverage data
	coverDir, err := os.MkdirTemp("", "hookflow-e2e-cover-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create coverage dir: %v\n", err)
		os.Exit(1)
	}
	globalCoverDir = coverDir

	// 3. Build the coverage-instrumented binary
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binDir := filepath.Join(repoRoot, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bin dir: %v\n", err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(binDir, "hookflow-cover"+ext)

	buildCmd := exec.Command("go", "build", "-cover", "-coverpkg=./...",
		"-o", binaryPath, "./cmd/hookflow")
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build coverage binary: %v\n", err)
		os.Exit(1)
	}

	// 4. Run all tests
	exitCode := m.Run()

	// 5. Merge and report coverage
	mergeCoverage()

	// 6. Cleanup
	_ = os.RemoveAll(globalCoverDir)
	_ = os.Remove(binaryPath)

	os.Exit(exitCode)
}

// findRepoRoot walks up from the current working directory to find the repo root
// (directory containing go.mod).
func findRepoRoot() (string, error) {
	// Start from this source file's location
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not determine caller location")
	}
	dir := filepath.Dir(filename)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

// mergeCoverage merges all per-test coverage subdirectories, converts to text
// format, and prints the coverage summary.
func mergeCoverage() {
	inputDirs := collectCoverDirs(globalCoverDir)
	if len(inputDirs) == 0 {
		fmt.Fprintln(os.Stderr, "No coverage data collected")
		return
	}

	mergedDir := filepath.Join(globalCoverDir, "merged")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create merged dir: %v\n", err)
		return
	}

	// Merge all coverage data
	mergeCmd := exec.Command("go", "tool", "covdata", "merge",
		"-i="+strings.Join(inputDirs, ","),
		"-o="+mergedDir)
	mergeCmd.Dir = repoRoot
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Coverage merge failed: %v\n%s\n", err, out)
		return
	}

	// Convert to text format for CI tooling
	coverageOut := filepath.Join(repoRoot, "e2e-coverage.out")
	textCmd := exec.Command("go", "tool", "covdata", "textfmt",
		"-i="+mergedDir,
		"-o="+coverageOut)
	textCmd.Dir = repoRoot
	if out, err := textCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Coverage textfmt failed: %v\n%s\n", err, out)
		return
	}

	// Print overall percentage
	pctCmd := exec.Command("go", "tool", "covdata", "percent", "-i="+mergedDir)
	pctCmd.Dir = repoRoot
	pctOut, err := pctCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Coverage percent failed: %v\n%s\n", err, pctOut)
		return
	}

	fmt.Fprintf(os.Stdout, "\n════════════════════════════════════════\n")
	fmt.Fprintf(os.Stdout, "  E2E Binary Coverage Report\n")
	fmt.Fprintf(os.Stdout, "════════════════════════════════════════\n")
	fmt.Fprintf(os.Stdout, "%s", pctOut)
	fmt.Fprintf(os.Stdout, "\nCoverage profile written to: %s\n", coverageOut)
	fmt.Fprintf(os.Stdout, "════════════════════════════════════════\n")
}

// collectCoverDirs finds all subdirectories of root that contain coverage data
// files (covcounters.* or covmeta.*). Excludes the "merged" directory.
func collectCoverDirs(root string) []string {
	var dirs []string
	seen := make(map[string]bool)

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "merged" {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, "covcounters.") || strings.HasPrefix(name, "covmeta.") {
			dir := filepath.Dir(path)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
		return nil
	})

	return dirs
}
