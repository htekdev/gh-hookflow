//go:build e2etest

package event

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

func init() {
	defaultGitProvider = &envGitProvider{}
}

// envGitProvider returns git context from environment variables so E2E tests
// can control commit/push event data without a real git repository.
//
// Environment variables:
//
//	HOOKFLOW_FAKE_GIT_BRANCH        - current branch (default: "main")
//	HOOKFLOW_FAKE_GIT_AUTHOR        - commit author (default: "E2E Test")
//	HOOKFLOW_FAKE_GIT_STAGED_FILES  - JSON array of {path,status} objects
//	HOOKFLOW_FAKE_GIT_PENDING_FILES - JSON array of {path,status} objects
//	HOOKFLOW_FAKE_GIT_REMOTE        - remote URL (default: "origin")
//	HOOKFLOW_FAKE_GIT_AHEAD         - commits ahead (default: "0")
//	HOOKFLOW_FAKE_GIT_BEHIND        - commits behind (default: "0")
type envGitProvider struct{}

func (e *envGitProvider) GetBranch(cwd string) string {
	if v := os.Getenv("HOOKFLOW_FAKE_GIT_BRANCH"); v != "" {
		return v
	}
	return "main"
}

func (e *envGitProvider) GetAuthor(cwd string) string {
	if v := os.Getenv("HOOKFLOW_FAKE_GIT_AUTHOR"); v != "" {
		return v
	}
	return "E2E Test"
}

func (e *envGitProvider) GetStagedFiles(cwd string) []schema.FileStatus {
	return parseFileStatusEnv("HOOKFLOW_FAKE_GIT_STAGED_FILES")
}

func (e *envGitProvider) GetPendingFiles(cwd string, command string) []schema.FileStatus {
	return parseFileStatusEnv("HOOKFLOW_FAKE_GIT_PENDING_FILES")
}

func (e *envGitProvider) GetRemote(cwd string) string {
	if v := os.Getenv("HOOKFLOW_FAKE_GIT_REMOTE"); v != "" {
		return v
	}
	return "origin"
}

func (e *envGitProvider) GetAheadBehind(cwd string) (ahead, behind int) {
	if v := os.Getenv("HOOKFLOW_FAKE_GIT_AHEAD"); v != "" {
		var a int
		for _, c := range v {
			a = a*10 + int(c-'0')
		}
		ahead = a
	}
	if v := os.Getenv("HOOKFLOW_FAKE_GIT_BEHIND"); v != "" {
		var b int
		for _, c := range v {
			b = b*10 + int(c-'0')
		}
		behind = b
	}
	return
}

func parseFileStatusEnv(envVar string) []schema.FileStatus {
	v := os.Getenv(envVar)
	if v == "" {
		return nil
	}

	// Support simple comma-separated paths (e.g., "file1.txt,file2.go")
	if !strings.HasPrefix(v, "[") {
		paths := strings.Split(v, ",")
		files := make([]schema.FileStatus, 0, len(paths))
		for _, p := range paths {
			p = strings.TrimSpace(p)
			if p != "" {
				files = append(files, schema.FileStatus{Path: p, Status: "A"})
			}
		}
		return files
	}

	// JSON array format
	var files []schema.FileStatus
	if err := json.Unmarshal([]byte(v), &files); err != nil {
		return nil
	}
	return files
}
