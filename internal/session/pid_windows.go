//go:build windows

package session

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = modkernel32.NewProc("Process32FirstW")
	procProcess32Next            = modkernel32.NewProc("Process32NextW")
)

const (
	TH32CS_SNAPPROCESS               = 0x00000002
	MAX_PATH                         = 260
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

type processEntry32 struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	PriClassBase      int32
	Flags             uint32
	ExeFile           [MAX_PATH]uint16
}

// GetCopilotPID walks up the process tree to find the "copilot" process.
// Returns the PID of the copilot process or an error if not found.
func GetCopilotPID() (int, error) {
	processes, err := getProcessList()
	if err != nil {
		return 0, fmt.Errorf("failed to get process list: %w", err)
	}

	// Build a map of PID -> parent PID and PID -> name
	parentMap := make(map[uint32]uint32)
	nameMap := make(map[uint32]string)
	for _, p := range processes {
		parentMap[p.ProcessID] = p.ParentProcessID
		nameMap[p.ProcessID] = syscall.UTF16ToString(p.ExeFile[:])
	}

	// Walk up from current process
	currentPID := uint32(os.Getpid())
	visited := make(map[uint32]bool)

	for currentPID != 0 {
		if visited[currentPID] {
			break // Avoid infinite loops
		}
		visited[currentPID] = true

		name := strings.ToLower(nameMap[currentPID])
		// Check for "copilot" in the process name (handles copilot.exe, GitHub Copilot, etc.)
		if strings.Contains(name, "copilot") {
			return int(currentPID), nil
		}

		currentPID = parentMap[currentPID]
	}

	return 0, ErrCopilotNotFound
}

// getProcessList returns a list of all running processes
func getProcessList() ([]processEntry32, error) {
	handle, _, err := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(handle)) }()

	var processes []processEntry32
	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, err := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed: %w", err)
	}

	for {
		processes = append(processes, entry)
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ := procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return processes, nil
}

// processExistsCheck checks if a process exists on Windows
func processExistsCheck(_ *os.Process, pid int) bool {
	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(handle)
	return true
}
