package event

import (
	"testing"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestParseGitStatus tests parsing of git diff --name-status output
func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []schema.FileStatus
	}{
		{
			name:   "empty",
			output: "",
			want:   nil,
		},
		{
			name:   "single modified",
			output: "M\tsrc/app.ts",
			want:   []schema.FileStatus{{Path: "src/app.ts", Status: "modified"}},
		},
		{
			name:   "single added",
			output: "A\tnew-file.ts",
			want:   []schema.FileStatus{{Path: "new-file.ts", Status: "added"}},
		},
		{
			name:   "single deleted",
			output: "D\told-file.ts",
			want:   []schema.FileStatus{{Path: "old-file.ts", Status: "deleted"}},
		},
		{
			name:   "renamed",
			output: "R\told.ts\tnew.ts",
			want:   []schema.FileStatus{{Path: "old.ts", Status: "renamed"}},
		},
		{
			name:   "multiple files",
			output: "M\tfile1.ts\nA\tfile2.ts\nD\tfile3.ts",
			want: []schema.FileStatus{
				{Path: "file1.ts", Status: "modified"},
				{Path: "file2.ts", Status: "added"},
				{Path: "file3.ts", Status: "deleted"},
			},
		},
		{
			name:   "with trailing newline",
			output: "M\tfile.ts\n",
			want:   []schema.FileStatus{{Path: "file.ts", Status: "modified"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGitStatus(tt.output)
			if len(got) != len(tt.want) {
				t.Errorf("parseGitStatus() returned %d files, want %d", len(got), len(tt.want))
				return
			}
			for i, f := range got {
				if f.Path != tt.want[i].Path || f.Status != tt.want[i].Status {
					t.Errorf("parseGitStatus()[%d] = %+v, want %+v", i, f, tt.want[i])
				}
			}
		})
	}
}

// TestParsePorcelainStatus tests parsing of git status --porcelain output
func TestParsePorcelainStatus(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []schema.FileStatus
	}{
		{
			name:   "empty",
			output: "",
			want:   nil,
		},
		{
			name:   "modified in working tree",
			output: " M src/app.ts", // XY format: space=not staged, M=modified in worktree
			want:   []schema.FileStatus{{Path: "src/app.ts", Status: "modified"}},
		},
		{
			name:   "modified in index",
			output: "M  src/app.ts", // XY format: M=staged, space=clean worktree
			want:   []schema.FileStatus{{Path: "src/app.ts", Status: "modified"}},
		},
		{
			name:   "added",
			output: "A  new-file.ts",
			want:   []schema.FileStatus{{Path: "new-file.ts", Status: "added"}},
		},
		{
			name:   "deleted",
			output: "D  old-file.ts",
			want:   []schema.FileStatus{{Path: "old-file.ts", Status: "deleted"}},
		},
		{
			name:   "untracked",
			output: "?? untracked.ts",
			want:   []schema.FileStatus{{Path: "untracked.ts", Status: "added"}},
		},
		{
			name:   "renamed",
			output: "R  old.ts -> new.ts",
			want:   []schema.FileStatus{{Path: "new.ts", Status: "renamed"}},
		},
		{
			name:   "multiple files",
			output: " M file1.ts\nA  file2.ts\n?? file3.ts",
			want: []schema.FileStatus{
				{Path: "file1.ts", Status: "modified"},
				{Path: "file2.ts", Status: "added"},
				{Path: "file3.ts", Status: "added"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePorcelainStatus(tt.output)
			if len(got) != len(tt.want) {
				t.Errorf("parsePorcelainStatus() returned %d files, want %d", len(got), len(tt.want))
				return
			}
			for i, f := range got {
				if f.Path != tt.want[i].Path || f.Status != tt.want[i].Status {
					t.Errorf("parsePorcelainStatus()[%d] = %+v, want %+v", i, f, tt.want[i])
				}
			}
		})
	}
}

// TestMatchGitAddPattern tests git add pattern matching
func TestMatchGitAddPattern(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		pattern  string
		want     bool
	}{
		// Dot matches everything
		{"dot matches any", "src/app.ts", ".", true},
		{"dot matches nested", "a/b/c/d.ts", ".", true},

		// All flag matches everything
		{"-A matches any", "src/app.ts", "-A", true},
		{"--all matches any", "src/app.ts", "--all", true},

		// Exact match
		{"exact match", "file.ts", "file.ts", true},
		{"exact no match", "file.ts", "other.ts", false},

		// Glob patterns
		{"glob *.ts match", "app.ts", "*.ts", true},
		{"glob *.ts no match", "app.js", "*.ts", false},
		{"glob src/*.ts match", "src/app.ts", "src/*.ts", true}, // matchGitAddPattern uses string contains

		// Directory patterns
		{"dir match", "src/app.ts", "src/", true},
		{"dir match no slash", "src/app.ts", "src", true},
		{"dir no match", "lib/app.ts", "src/", false},

		// Path contains
		{"path contains", "src/components/Button.tsx", "components", true},
		{"path not contains", "src/utils/helper.ts", "components", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGitAddPattern(tt.filePath, tt.pattern, "/test")
			if got != tt.want {
				t.Errorf("matchGitAddPattern(%q, %q) = %v, want %v", tt.filePath, tt.pattern, got, tt.want)
			}
		})
	}
}

// TestMockGitProvider tests the mock provider
func TestMockGitProvider(t *testing.T) {
	mock := &MockGitProvider{
		Branch: "feature",
		Author: "test@test.com",
		StagedFiles: []schema.FileStatus{
			{Path: "file.ts", Status: "modified"},
		},
		PendingFiles: []schema.FileStatus{
			{Path: "new.ts", Status: "added"},
		},
		Remote: "upstream",
		Ahead:  5,
		Behind: 2,
	}

	if mock.GetBranch("/any") != "feature" {
		t.Error("GetBranch mismatch")
	}
	if mock.GetAuthor("/any") != "test@test.com" {
		t.Error("GetAuthor mismatch")
	}
	if len(mock.GetStagedFiles("/any")) != 1 {
		t.Error("GetStagedFiles mismatch")
	}
	if len(mock.GetPendingFiles("/any", "git add .")) != 1 {
		t.Error("GetPendingFiles mismatch")
	}
	if mock.GetRemote("/any") != "upstream" {
		t.Error("GetRemote mismatch")
	}
	ahead, behind := mock.GetAheadBehind("/any")
	if ahead != 5 || behind != 2 {
		t.Error("GetAheadBehind mismatch")
	}
}

// TestMockGitProviderDefaults tests default values
func TestMockGitProviderDefaults(t *testing.T) {
	mock := &MockGitProvider{} // Empty mock

	// Remote should default to "origin"
	if mock.GetRemote("/any") != "origin" {
		t.Errorf("GetRemote() = %q, want 'origin'", mock.GetRemote("/any"))
	}

	// Others should return zero values
	if mock.GetBranch("/any") != "" {
		t.Error("GetBranch should return empty string")
	}
	if mock.GetAuthor("/any") != "" {
		t.Error("GetAuthor should return empty string")
	}
	if mock.GetStagedFiles("/any") != nil {
		t.Error("GetStagedFiles should return nil")
	}
}

// TestParseCount tests the parseCount helper function
func TestParseCount(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"42", 42, false},
		{"100", 100, false},
		{"3", 3, false},
	}

	for _, tt := range tests {
		got, err := parseCount(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseCount(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseCount(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// TestRealGitProviderNonGitDir tests RealGitProvider in a non-git directory
func TestRealGitProviderNonGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	provider := &RealGitProvider{}

	// These methods should return empty/zero values gracefully in a non-git directory
	if branch := provider.GetBranch(tmpDir); branch != "" {
		t.Errorf("GetBranch in non-git dir = %q, want empty", branch)
	}
	if files := provider.GetStagedFiles(tmpDir); files != nil {
		t.Errorf("GetStagedFiles in non-git dir = %v, want nil", files)
	}
	if files := provider.GetPendingFiles(tmpDir, "git add ."); files != nil {
		t.Errorf("GetPendingFiles in non-git dir = %v, want nil", files)
	}
	ahead, behind := provider.GetAheadBehind(tmpDir)
	if ahead != 0 || behind != 0 {
		t.Errorf("GetAheadBehind in non-git dir = (%d, %d), want (0, 0)", ahead, behind)
	}
	// GetRemote defaults to "origin" on error
	remote := provider.GetRemote(tmpDir)
	if remote != "origin" {
		t.Errorf("GetRemote in non-git dir = %q, want 'origin'", remote)
	}
}
