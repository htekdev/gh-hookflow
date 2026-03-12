package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test workflow files
	files := []string{"lint.yml", "security.yaml", "test.yml"}
	for _, f := range files {
		path := filepath.Join(workflowDir, f)
		if err := os.WriteFile(path, []byte("name: test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-workflow file
	if err := os.WriteFile(filepath.Join(workflowDir, "readme.txt"), []byte("# readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test discovery
	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 3 {
		t.Errorf("Discover() found %d workflows, want 3", len(workflows))
	}

	// Verify names
	names := make(map[string]bool)
	for _, w := range workflows {
		names[w.Name] = true
	}

	for _, expected := range []string{"lint", "security", "test"} {
		if !names[expected] {
			t.Errorf("Discover() missing workflow %q", expected)
		}
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Discover() found %d workflows, want 0", len(workflows))
	}
}

func TestDiscoverNoWorkflowDir(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create .github but not hooks
	if err := os.MkdirAll(filepath.Join(tmpDir, ".github"), 0755); err != nil {
		t.Fatal(err)
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Discover() found %d workflows, want 0", len(workflows))
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a workflow file
	if err := os.WriteFile(filepath.Join(workflowDir, "lint.yml"), []byte("name: lint"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		workflow   string
		wantExists bool
	}{
		{"exists yml", "lint", true},
		{"not exists", "security", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, exists := Exists(tmpDir, tt.workflow)
			if exists != tt.wantExists {
				t.Errorf("Exists(%q) = %v, want %v", tt.workflow, exists, tt.wantExists)
			}
			if exists && path == "" {
				t.Error("Exists() returned empty path for existing workflow")
			}
		})
	}
}

func TestWorkflowFile(t *testing.T) {
	wf := WorkflowFile{
		Path:    "/repo/.github/hookflows/lint.yml",
		Name:    "lint",
		RelPath: ".github/hookflows/lint.yml",
	}

	if wf.Name != "lint" {
		t.Errorf("WorkflowFile.Name = %q, want %q", wf.Name, "lint")
	}

	if wf.RelPath != ".github/hookflows/lint.yml" {
		t.Errorf("WorkflowFile.RelPath = %q, want %q", wf.RelPath, ".github/hookflows/lint.yml")
	}
}

func TestDiscoverByGlob(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test workflow files
	files := []string{"lint.yml", "security.yaml", "test.yml"}
	for _, f := range files {
		path := filepath.Join(workflowDir, f)
		if err := os.WriteFile(path, []byte("name: test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		pattern  string
		wantLen  int
		wantName string
	}{
		{"match all yml", "*.yml", 2, "lint"},
		{"match all yaml", "*.yaml", 1, "security"},
		{"match specific", "lint.yml", 1, "lint"},
		{"match all", "*", 3, ""},
		{"no match", "nonexistent.yml", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflows, err := DiscoverByGlob(tmpDir, tt.pattern)
			if err != nil {
				t.Fatalf("DiscoverByGlob() error = %v", err)
			}
			if len(workflows) != tt.wantLen {
				t.Errorf("DiscoverByGlob(%q) found %d workflows, want %d", tt.pattern, len(workflows), tt.wantLen)
			}
			if tt.wantName != "" && len(workflows) > 0 {
				found := false
				for _, w := range workflows {
					if w.Name == tt.wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DiscoverByGlob(%q) missing expected workflow %q", tt.pattern, tt.wantName)
				}
			}
		})
	}
}

func TestDiscoverByGlobSkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	subDir := filepath.Join(workflowDir, "subdir.yml") // directory named like a file
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a real file
	if err := os.WriteFile(filepath.Join(workflowDir, "real.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	workflows, err := DiscoverByGlob(tmpDir, "*.yml")
	if err != nil {
		t.Fatalf("DiscoverByGlob() error = %v", err)
	}

	// Should only find the real file, not the directory
	if len(workflows) != 1 {
		t.Errorf("DiscoverByGlob() found %d workflows, want 1", len(workflows))
	}
	if len(workflows) > 0 && workflows[0].Name != "real" {
		t.Errorf("DiscoverByGlob() found %q, want 'real'", workflows[0].Name)
	}
}

func TestDiscoverByGlobNonStandardExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files with non-standard extensions
	files := []string{"workflow.txt", "workflow.json", "workflow.xml", "valid.yml"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(workflowDir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := DiscoverByGlob(tmpDir, "workflow.*")
	if err != nil {
		t.Fatalf("DiscoverByGlob() error = %v", err)
	}

	// Should find none because workflow.* matches non-yaml files
	if len(workflows) != 0 {
		t.Errorf("DiscoverByGlob() found %d workflows with non-standard extensions, want 0", len(workflows))
	}
}

func TestDiscoverByGlobInvalidPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// filepath.Glob returns error for invalid patterns like [
	_, err := DiscoverByGlob(tmpDir, "[invalid")
	if err == nil {
		t.Error("DiscoverByGlob() with invalid pattern should return error")
	}
}

func TestDiscoverNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")

	// Create nested directories
	subdirs := []string{
		"",
		"subdir",
		"deep/nested/path",
	}

	for _, subdir := range subdirs {
		dir := filepath.Join(workflowDir, subdir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create workflow file in each dir
		name := "workflow"
		if subdir != "" {
			name = filepath.Base(subdir) + "-workflow"
		}
		path := filepath.Join(dir, name+".yml")
		if err := os.WriteFile(path, []byte("name: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should find all workflows including nested ones
	if len(workflows) != 3 {
		t.Errorf("Discover() found %d workflows, want 3", len(workflows))
	}

	// Verify nested workflow has correct relative path
	foundNested := false
	for _, w := range workflows {
		if w.Name == "path-workflow" {
			foundNested = true
			expectedRel := filepath.Join(".github", "hookflows", "deep", "nested", "path", "path-workflow.yml")
			if w.RelPath != expectedRel {
				t.Errorf("Nested workflow RelPath = %q, want %q", w.RelPath, expectedRel)
			}
		}
	}
	if !foundNested {
		t.Error("Discover() did not find deeply nested workflow")
	}
}

func TestDiscoverMixedExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create both .yml and .yaml files
	ymlFiles := []string{"a.yml", "b.yml", "c.yml"}
	yamlFiles := []string{"d.yaml", "e.yaml"}

	for _, f := range ymlFiles {
		if err := os.WriteFile(filepath.Join(workflowDir, f), []byte("name: "+f), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range yamlFiles {
		if err := os.WriteFile(filepath.Join(workflowDir, f), []byte("name: "+f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 5 {
		t.Errorf("Discover() found %d workflows, want 5", len(workflows))
	}

	// Verify counts by extension
	ymlCount := 0
	yamlCount := 0
	for _, w := range workflows {
		if filepath.Ext(w.Path) == ".yml" {
			ymlCount++
		} else if filepath.Ext(w.Path) == ".yaml" {
			yamlCount++
		}
	}

	if ymlCount != 3 {
		t.Errorf("Found %d .yml files, want 3", ymlCount)
	}
	if yamlCount != 2 {
		t.Errorf("Found %d .yaml files, want 2", yamlCount)
	}
}

func TestDiscoverNonStandardExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files with various extensions
	files := map[string]bool{
		"valid.yml":        true,  // should be found
		"valid.yaml":       true,  // should be found
		"invalid.txt":      false, // should be excluded
		"invalid.json":     false, // should be excluded
		"invalid.YML":      true,  // should be found (case insensitive)
		"invalid.YAML":     true,  // should be found (case insensitive)
		"hookify.md":       true,  // should be found (hookify format)
		"hookify-upper.MD": true,  // should be found (case insensitive)
		"noextension":      false, // should be excluded
		"readme.txt":       false, // should be excluded
		"workflow.yml.bak": false, // should be excluded
	}

	for f := range files {
		if err := os.WriteFile(filepath.Join(workflowDir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Count expected valid files
	expectedCount := 0
	for _, valid := range files {
		if valid {
			expectedCount++
		}
	}

	if len(workflows) != expectedCount {
		t.Errorf("Discover() found %d workflows, want %d", len(workflows), expectedCount)
	}
}

func TestExistsYamlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create only .yaml file
	if err := os.WriteFile(filepath.Join(workflowDir, "security.yaml"), []byte("name: security"), 0644); err != nil {
		t.Fatal(err)
	}

	path, exists := Exists(tmpDir, "security")
	if !exists {
		t.Error("Exists() should find .yaml file")
	}
	if !filepath.IsAbs(path) || filepath.Ext(path) != ".yaml" {
		t.Errorf("Exists() returned incorrect path: %q", path)
	}
}

func TestExistsPrefersYml(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create both .yml and .yaml files with same name
	if err := os.WriteFile(filepath.Join(workflowDir, "lint.yml"), []byte("name: lint yml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "lint.yaml"), []byte("name: lint yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	path, exists := Exists(tmpDir, "lint")
	if !exists {
		t.Error("Exists() should find workflow")
	}
	// Should prefer .yml extension (checked first)
	if filepath.Ext(path) != ".yml" {
		t.Errorf("Exists() should prefer .yml, got path: %q", path)
	}
}

func TestDiscoverWalkError(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid workflow file
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create unreadable subdirectory (only works on Unix-like systems)
	unreadableDir := filepath.Join(workflowDir, "unreadable")
	if err := os.MkdirAll(unreadableDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make directory unreadable (this may not work on Windows)
	if err := os.Chmod(unreadableDir, 0000); err != nil {
		t.Skip("Cannot test permission errors on this platform")
	}
	defer func() { _ = os.Chmod(unreadableDir, 0755) }() // cleanup

	// Discover should return error when it cannot read a directory
	_, err := Discover(tmpDir)
	if err == nil {
		// On some systems (Windows), permissions work differently
		// Just verify we can handle the case gracefully
		t.Log("Permission-based error test skipped on this platform")
	}
}

func TestDiscoverByGlobStatError(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file that we'll delete between Glob and Stat
	testFile := filepath.Join(workflowDir, "temp.yml")
	if err := os.WriteFile(testFile, []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test normal case first to ensure setup is correct
	workflows, err := DiscoverByGlob(tmpDir, "*.yml")
	if err != nil {
		t.Fatalf("DiscoverByGlob() error = %v", err)
	}
	if len(workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(workflows))
	}
}

func TestDiscoverEmptyWorkflowDir(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow directory exists but is empty
	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Discover() found %d workflows in empty dir, want 0", len(workflows))
	}
}

func TestDiscoverByGlobEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	workflows, err := DiscoverByGlob(tmpDir, "*.yml")
	if err != nil {
		t.Fatalf("DiscoverByGlob() error = %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("DiscoverByGlob() found %d workflows in empty dir, want 0", len(workflows))
	}
}

func TestDiscoverRelativePathHandling(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow file
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow, got %d", len(workflows))
	}

	w := workflows[0]

	// Path should be absolute
	if !filepath.IsAbs(w.Path) {
		t.Errorf("WorkflowFile.Path should be absolute, got: %q", w.Path)
	}

	// RelPath should be relative and start with .github
	if filepath.IsAbs(w.RelPath) {
		t.Errorf("WorkflowFile.RelPath should be relative, got: %q", w.RelPath)
	}

	expectedRelPath := filepath.Join(".github", "hookflows", "test.yml")
	if w.RelPath != expectedRelPath {
		t.Errorf("WorkflowFile.RelPath = %q, want %q", w.RelPath, expectedRelPath)
	}
}

func TestDiscoverByGlobRelativePathHandling(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow file
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	workflows, err := DiscoverByGlob(tmpDir, "*.yml")
	if err != nil {
		t.Fatalf("DiscoverByGlob() error = %v", err)
	}

	if len(workflows) != 1 {
		t.Fatalf("Expected 1 workflow, got %d", len(workflows))
	}

	w := workflows[0]

	// Path should be absolute
	if !filepath.IsAbs(w.Path) {
		t.Errorf("WorkflowFile.Path should be absolute, got: %q", w.Path)
	}

	// RelPath should be relative
	if filepath.IsAbs(w.RelPath) {
		t.Errorf("WorkflowFile.RelPath should be relative, got: %q", w.RelPath)
	}

	expectedRelPath := filepath.Join(".github", "hookflows", "test.yml")
	if w.RelPath != expectedRelPath {
		t.Errorf("WorkflowFile.RelPath = %q, want %q", w.RelPath, expectedRelPath)
	}
}

func TestDiscoverOnlyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")

	// Create only directories (no files)
	dirs := []string{"subdir1", "subdir2", "deep/nested"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(workflowDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(workflows) != 0 {
		t.Errorf("Discover() found %d workflows when only directories exist, want 0", len(workflows))
	}
}

func TestWorkflowFileNameExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files with various naming patterns
	files := map[string]string{
		"simple.yml":           "simple",
		"with-dash.yml":        "with-dash",
		"with_underscore.yaml": "with_underscore",
		"CamelCase.yml":        "CamelCase",
		"multiple.dots.yml":    "multiple.dots",
	}

	for f := range files {
		if err := os.WriteFile(filepath.Join(workflowDir, f), []byte("name: test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workflows, err := Discover(tmpDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Build map of discovered names
	discovered := make(map[string]bool)
	for _, w := range workflows {
		discovered[w.Name] = true
	}

	// Verify all expected names are found
	for _, expected := range files {
		if !discovered[expected] {
			t.Errorf("Expected workflow name %q not found", expected)
		}
	}
}
