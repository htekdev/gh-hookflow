//go:build !windows

package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// GetCopilotPID walks up the process tree to find the "copilot" process.
// Returns the PID of the copilot process or an error if not found.
func GetCopilotPID() (int, error) {
	currentPID := os.Getpid()
	visited := make(map[int]bool)

	for currentPID > 1 {
		if visited[currentPID] {
			break // Avoid infinite loops
		}
		visited[currentPID] = true

		name, err := getProcessName(currentPID)
		if err != nil {
			// If we can't get the name, try to continue with parent
			parentPID, parentErr := getParentPID(currentPID)
			if parentErr != nil {
				break
			}
			currentPID = parentPID
			continue
		}

		// Check for "copilot" in the process name (handles copilot, GitHub Copilot, etc.)
		if strings.Contains(strings.ToLower(name), "copilot") {
			return currentPID, nil
		}

		parentPID, err := getParentPID(currentPID)
		if err != nil {
			break
		}
		currentPID = parentPID
	}

	return 0, ErrCopilotNotFound
}

// getProcessName returns the name of a process by PID
func getProcessName(pid int) (string, error) {
	// Try /proc/{pid}/comm first (shorter, just the command name)
	commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
	data, err := os.ReadFile(commPath)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Fallback to /proc/{pid}/stat
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err = os.ReadFile(statPath)
	if err != nil {
		return "", fmt.Errorf("failed to read process info: %w", err)
	}

	// Parse stat: pid (comm) state ...
	// Find the command name between parentheses
	start := strings.Index(string(data), "(")
	end := strings.LastIndex(string(data), ")")
	if start == -1 || end == -1 || start >= end {
		return "", fmt.Errorf("failed to parse process stat")
	}

	return string(data[start+1 : end]), nil
}

// getParentPID returns the parent PID of a process
func getParentPID(pid int) (int, error) {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read process stat: %w", err)
	}

	// Parse stat: pid (comm) state ppid ...
	// Find the end of the command name (last ')') and parse from there
	end := strings.LastIndex(string(data), ")")
	if end == -1 {
		return 0, fmt.Errorf("failed to parse process stat")
	}

	// Fields after the command name: state ppid ...
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 2 {
		return 0, fmt.Errorf("failed to parse parent PID")
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse parent PID: %w", err)
	}

	return ppid, nil
}

// processExistsCheck checks if a process exists on Unix
func processExistsCheck(process *os.Process, _ int) bool {
	// On Unix, sending signal 0 checks if the process exists without affecting it
	err := process.Signal(syscall.Signal(0))
	return err == nil
}
