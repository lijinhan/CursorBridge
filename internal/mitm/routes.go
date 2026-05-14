package mitm

import "strings"

// isTelemetryPath returns true for api2 paths that send user data
// (conversation content, usage metrics, analytics, crash reports) to
// Cursor's servers.
func isTelemetryPath(path string) bool {
	if !strings.HasPrefix(path, "/aiserver.v1.AiService/") {
		return false
	}
	method := path[len("/aiserver.v1.AiService/"):]
	telemetryPrefixes := []string{
		"Report",
		"Track",
		"Log",
		"Submit",
		"Send",
		"Record",
		"Ingest",
		"Collect",
		"Upload",
		"Post",
		"Capture",
		"Emit",
		"Flush",
		"Batch",
		"Put",
		"Store",
		"CreateFeed",
		"GetSentry",
	}
	for _, prefix := range telemetryPrefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	telemetryExact := map[string]struct{}{
		"GetTelemetryConfig":          {},
		"GetAnalyticsConfig":          {},
		"GetCrashReportConfig":        {},
		"GetSentryDsn":                {},
		"GetPostHogApiKey":            {},
		"GetAmplitudeApiKey":          {},
		"GetMixpanelApiKey":           {},
		"GetSegmentWriteKey":          {},
		"GetLaunchDarklyClientId":     {},
		"GetStatsigFeatureGates":      {},
		"GetDynamicConfig":            {},
		"GetExperimentConfig":         {},
		"GetABTestConfig":             {},
		"GetPrivacyConfig":            {},
		"GetNotificationConfig":       {},
		"GetExtensionRecommendations": {},
	}
	if _, ok := telemetryExact[method]; ok {
		return true
	}
	return false
}

// isEmptyMockPath matches service paths that should return an empty 200
// instead of being forwarded upstream.
func isEmptyMockPath(path string) bool {
	if strings.HasPrefix(path, "/aiserver.v1.DashboardService/") {
		return true
	}
	if strings.HasPrefix(path, "/aiserver.v1.AnalyticsService/") {
		return true
	}
	if strings.HasPrefix(path, "/aiserver.v1.BackgroundComposerService/") {
		return true
	}
	return false
}
