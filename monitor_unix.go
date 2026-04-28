//go:build !windows

package gateway

import (
	"os"
	"time"
)

// monitorParent exits the gateway subprocess if the parent (squadron)
// goes away. On Unix the kernel reparents orphaned children to init
// (PID 1), which we use as the death signal.
func monitorParent() {
	ppid := os.Getppid()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if parentLost(ppid, os.Getppid()) {
			os.Exit(0)
		}
	}
}

// parentLost is the orphan check, factored out of the polling loop so
// it can be unit-tested. On Unix, "lost" means either the parent's PID
// changed (the original squadron process is gone, and the kernel
// either reparented us or PIDs were reused) or the current parent is
// init (PID 1, the universal orphan-adoption sentinel).
func parentLost(initialPPID, currentPPID int) bool {
	return currentPPID != initialPPID || currentPPID == 1
}
