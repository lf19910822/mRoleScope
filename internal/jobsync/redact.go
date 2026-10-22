package jobsync

import (
	"regexp"
	"strings"
)

const maxAuditMessageLength = 512

var sensitiveValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|token|secret|access[_-]?key|authorization)\s*([=:])\s*[^\s,;]+`),
	regexp.MustCompile(`(?i)(postgres(?:ql)?://)[^\s]+`),
	regexp.MustCompile(`(?i)(bearer)\s+[^\s,;]+`),
}

// SafeMessage is the only error text that should enter audit rows or logs.
// It is intentionally terse: raw API bodies and JD content must stay in COS.
func SafeMessage(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	for _, pattern := range sensitiveValuePatterns {
		value = pattern.ReplaceAllStringFunc(value, redactMatch)
	}
	value = strings.TrimSpace(spacePattern.ReplaceAllString(value, " "))
	if len(value) > maxAuditMessageLength {
		return value[:maxAuditMessageLength] + "…"
	}
	return value
}

func redactMatch(match string) string {
	lower := strings.ToLower(match)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return "[redacted-dsn]"
	}
	if strings.HasPrefix(lower, "bearer") {
		return "Bearer [redacted]"
	}
	if separator := strings.IndexAny(match, "=:"); separator >= 0 {
		return match[:separator+1] + "[redacted]"
	}
	return "[redacted]"
}
