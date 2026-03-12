package discover

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// WorkflowDir is the directory where agent workflows are stored
	WorkflowDir = ".github/hookflows"
)

// WorkflowFile represents a discovered workflow file
type WorkflowFile struct {
	Path     string // Full path to the file
	Name     string // Workflow name (filename without extension)
	RelPath  string // Relative path from root
}

// Discover finds all workflow files in the given directory
func Discover(rootDir string) ([]WorkflowFile, error) {
	workflowPath := filepath.Join(rootDir, WorkflowDir)
	
	// Check if workflow directory exists
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return []WorkflowFile{}, nil
	}

	var workflows []WorkflowFile

	err := filepath.Walk(workflowPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .yml, .yaml, and .md files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" && ext != ".md" {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path
		}

		// Extract workflow name (filename without extension)
		name := strings.TrimSuffix(filepath.Base(path), ext)

		workflows = append(workflows, WorkflowFile{
			Path:    path,
			Name:    name,
			RelPath: relPath,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return workflows, nil
}

// DiscoverByGlob finds workflow files matching a glob pattern
func DiscoverByGlob(rootDir string, pattern string) ([]WorkflowFile, error) {
	workflowPath := filepath.Join(rootDir, WorkflowDir, pattern)
	
	matches, err := filepath.Glob(workflowPath)
	if err != nil {
		return nil, err
	}

	var workflows []WorkflowFile
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" && ext != ".md" {
			continue
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)

		workflows = append(workflows, WorkflowFile{
			Path:    path,
			Name:    name,
			RelPath: relPath,
		})
	}

	return workflows, nil
}

// Exists checks if a specific workflow file exists
func Exists(rootDir, workflowName string) (string, bool) {
	for _, ext := range []string{".yml", ".yaml", ".md"} {
		path := filepath.Join(rootDir, WorkflowDir, workflowName+ext)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
