package instance

import (
	"os"
	"testing"
)

// TestRealLifecycle is deliberately opt-in. Hosted and normal test runs must
// never install or launch a persistent CLIProxyAPI child.
func TestRealLifecycle(t *testing.T) {
	if os.Getenv("DUALPOOL_RUN_REAL_LIFECYCLE") != "1" {
		t.Skip("set DUALPOOL_RUN_REAL_LIFECYCLE=1 only after the pre-live CI gate")
	}
	t.Fatal("real lifecycle orchestration has not been authorized by its pre-live gate")
}
