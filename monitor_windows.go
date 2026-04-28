//go:build windows

package gateway

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// monitorParent exits the gateway subprocess if the parent (squadron)
// goes away. On Windows we hold a handle to the parent and block on
// its termination, falling back to PPID polling if we can't open the
// handle.
func monitorParent() {
	ppid := os.Getppid()

	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(ppid))
	if err != nil {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if os.Getppid() != ppid {
				os.Exit(0)
			}
		}
		return
	}
	defer windows.CloseHandle(handle)

	windows.WaitForSingleObject(handle, windows.INFINITE)
	os.Exit(0)
}
