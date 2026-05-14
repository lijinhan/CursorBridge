package agent

import (
	"math"
	"strings"
	"time"
)

// defaultRetryCount is used when the adapter's RetryCount is 0 (unset).
// 2 retries is a reasonable default for production: it handles transient
// failures (429 rate-limit, 502/503 gateway errors) without excessive delay.
const defaultRetryCount = 2

// defaultRetryIntervalMs is used when the adapter's RetryIntervalMs is 0 (unset).
const defaultRetryIntervalMs = 1000

// defaultTimeoutMs is used when the adapter's TimeoutMs is 0 (unset).
const defaultTimeoutMs = 300000 // 5 minutes

// retryableStatusCodes are HTTP status codes that warrant a retry.
var retryableStatusCodes = []int{429, 500, 502, 503, 504}

// effectiveRetryCount returns the configured retry count, or 0 if unset
// (meaning no retries — the caller must opt in).
func effectiveRetryCount(n int) int {
	if n <= 0 {
		return defaultRetryCount
	}
	return n
}

// effectiveRetryIntervalMs returns the configured retry interval, or the
// default if unset.
func effectiveRetryIntervalMs(n int) int {
	if n <= 0 {
		return defaultRetryIntervalMs
	}
	return n
}

// effectiveTimeoutMs returns the configured timeout, or the default if unset.
func effectiveTimeoutMs(n int) time.Duration {
	if n <= 0 {
		return time.Duration(defaultTimeoutMs) * time.Millisecond
	}
	return time.Duration(n) * time.Millisecond
}

// isRetryableStatusCode checks if an HTTP status code is in the retryable set.
func isRetryableStatusCode(code int) bool {
	for _, c := range retryableStatusCodes {
		if c == code {
			return true
		}
	}
	return false
}

// isRetryableNetworkError checks if an error message indicates a retryable
// network failure (connection reset, timeout, DNS failure, etc.).
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, pattern := range []string{
		"connection reset",
		"econnreset",
		"etimedout",
		"timeout",
		"enotfound",
		"econnrefused",
		"network error",
		"forcibly closed",
		"aborted",
		"eof",
		"busy",
		"try again later",
		"internalerror",
	} {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// computeBackoffDelay returns the delay for attempt i using exponential
// backoff, capped at 60 seconds.
func computeBackoffDelay(baseMs int, attempt int) time.Duration {
	ms := float64(baseMs) * math.Pow(2, float64(attempt))
	if ms > 60000 {
		ms = 60000
	}
	return time.Duration(ms) * time.Millisecond
}

