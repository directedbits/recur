//go:build windows

package processos

import (
	"golang.org/x/sys/windows"
)

// SendTermSignal terminates a process on Windows.
// Windows has no SIGTERM — we send CTRL_BREAK_EVENT first for console processes,
// then fall back to TerminateProcess if the handle can be opened.
func SendTermSignal(pid int) error {
	// Try graceful: CTRL_BREAK_EVENT to the process's console group
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid))
	if err == nil {
		return nil
	}

	// Fallback: hard kill via TerminateProcess
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, 1)
}

// defaultSocketPath returns the TCP address for the daemon on Windows.
func defaultSocketPath() (string, error) {
	return "localhost:19384", nil
}

// IsProcessAlive checks if a process is running by opening a handle to it.
func IsProcessAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}
	return exitCode == 259 // STILL_ACTIVE
}
