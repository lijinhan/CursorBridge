package agent

import (
	"testing"
	"time"
)

func TestEffectiveRetryCount(t *testing.T) {
	if got := effectiveRetryCount(0); got != 2 {
		t.Errorf("effectiveRetryCount(0) = %d, want 2", got)
	}
	if got := effectiveRetryCount(-1); got != 2 {
		t.Errorf("effectiveRetryCount(-1) = %d, want 2", got)
	}
	if got := effectiveRetryCount(3); got != 3 {
		t.Errorf("effectiveRetryCount(3) = %d, want 3", got)
	}
}

func TestEffectiveRetryIntervalMs(t *testing.T) {
	if got := effectiveRetryIntervalMs(0); got != 1000 {
		t.Errorf("effectiveRetryIntervalMs(0) = %d, want 1000", got)
	}
	if got := effectiveRetryIntervalMs(500); got != 500 {
		t.Errorf("effectiveRetryIntervalMs(500) = %d, want 500", got)
	}
}

func TestEffectiveTimeoutMs(t *testing.T) {
	if got := effectiveTimeoutMs(0); got != 300000*time.Millisecond {
		t.Errorf("effectiveTimeoutMs(0) = %v, want 300000ms", got)
	}
	if got := effectiveTimeoutMs(60000); got != 60000*time.Millisecond {
		t.Errorf("effectiveTimeoutMs(60000) = %v, want 60000ms", got)
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	retryable := []int{429, 500, 502, 503, 504}
	for _, code := range retryable {
		if !isRetryableStatusCode(code) {
			t.Errorf("isRetryableStatusCode(%d) = false, want true", code)
		}
	}
	notRetryable := []int{200, 400, 401, 403, 404, 408}
	for _, code := range notRetryable {
		if isRetryableStatusCode(code) {
			t.Errorf("isRetryableStatusCode(%d) = true, want false", code)
		}
	}
}

func TestComputeBackoffDelay(t *testing.T) {
	base := 1000
	if got := computeBackoffDelay(base, 0); got != 1000*time.Millisecond {
		t.Errorf("computeBackoffDelay(1000, 0) = %v, want 1000ms", got)
	}
	if got := computeBackoffDelay(base, 1); got != 2000*time.Millisecond {
		t.Errorf("computeBackoffDelay(1000, 1) = %v, want 2000ms", got)
	}
	if got := computeBackoffDelay(base, 2); got != 4000*time.Millisecond {
		t.Errorf("computeBackoffDelay(1000, 2) = %v, want 4000ms", got)
	}
	// Capped at 60s
	if got := computeBackoffDelay(base, 10); got != 60000*time.Millisecond {
		t.Errorf("computeBackoffDelay(1000, 10) = %v, want 60000ms (capped)", got)
	}
}
