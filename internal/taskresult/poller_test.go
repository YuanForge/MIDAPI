package taskresult

import (
	"testing"
	"time"
)

func TestCalcWorkerStaleTimeoutUsesMinimum(t *testing.T) {
	if got := calcWorkerStaleTimeout(60_000); got != minWorkerStaleTimeout {
		t.Fatalf("calcWorkerStaleTimeout short timeout = %s, want %s", got, minWorkerStaleTimeout)
	}
}

func TestCalcWorkerStaleTimeoutUsesChannelTimeoutWithGrace(t *testing.T) {
	if got := calcWorkerStaleTimeout(15 * 60 * 1000); got != 17*time.Minute {
		t.Fatalf("calcWorkerStaleTimeout 15m = %s, want 17m", got)
	}
}

func TestCalcWorkerStaleTimeoutCapsAtMaxAge(t *testing.T) {
	if got := calcWorkerStaleTimeout(3 * 60 * 60 * 1000); got != maxAge {
		t.Fatalf("calcWorkerStaleTimeout huge timeout = %s, want %s", got, maxAge)
	}
}
