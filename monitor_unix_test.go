//go:build !windows

package gateway

import "testing"

// TestParentLost pins the orphan-prevention semantics that
// monitorParent relies on: a gateway subprocess must exit when (a) the
// parent's pid has changed (squadron crashed and we got reparented,
// possibly to a process that just took the same pid), or (b) the
// kernel has reparented us to init (pid 1), the universal sign that
// our original parent is gone.
//
// Without this guarantee, a crashed squadron would leave gateway
// subprocesses running indefinitely — exactly the orphan-leak case
// the SDK is designed to prevent.
func TestParentLost(t *testing.T) {
	cases := []struct {
		name        string
		initial     int
		current     int
		wantOrphan  bool
		description string
	}{
		{
			name:        "parent unchanged, normal pid",
			initial:     1234,
			current:     1234,
			wantOrphan:  false,
			description: "happy path: the parent we started under is still our parent",
		},
		{
			name:        "parent pid changed",
			initial:     1234,
			current:     5678,
			wantOrphan:  true,
			description: "original squadron is gone — reparenting is the death signal",
		},
		{
			name:        "current parent is init",
			initial:     1234,
			current:     1,
			wantOrphan:  true,
			description: "the kernel reparented us to init — squadron is definitely gone",
		},
		{
			name:        "started under init and still under init",
			initial:     1,
			current:     1,
			wantOrphan:  true,
			description: "init is never a real squadron parent — treat as orphaned defensively",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parentLost(tc.initial, tc.current)
			if got != tc.wantOrphan {
				t.Fatalf("parentLost(%d, %d) = %v; want %v (%s)",
					tc.initial, tc.current, got, tc.wantOrphan, tc.description)
			}
		})
	}
}
