package usage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ollamaCloudUsagePlanRE = regexp.MustCompile(`(?is)<h2[^>]*>.*?<span[^>]*>\s*Cloud Usage\s*</span\s*>.*?<span[^>]*>\s*([^<]+?)\s*</span\s*>`)
	ollamaSessionUsageRE   = regexp.MustCompile(`(?is)<span[^>]*>\s*Session usage\s*</span\s*>.*?<span[^>]*>\s*([0-9]+(?:\.[0-9]+)?)%\s*used\s*</span\s*>.*?data-time="([^"]+)"`)
	ollamaWeeklyUsageRE    = regexp.MustCompile(`(?is)<span[^>]*>\s*Weekly usage\s*</span\s*>.*?<span[^>]*>\s*([0-9]+(?:\.[0-9]+)?)%\s*used\s*</span\s*>.*?data-time="([^"]+)"`)
	ollamaNotifyUsageRE    = regexp.MustCompile(`(?is)name="notify-usage-limits"[^>]*\bchecked\b`)
)

func ParseOllamaSettingsUsageHTML(data []byte, fetchedAt, expiresAt time.Time) (QuotaSummary, error) {
	html := string(data)

	sessionPercent, sessionResetsAt, err := parseUsageWindow(html, ollamaSessionUsageRE, "session")
	if err != nil {
		return QuotaSummary{}, err
	}
	weeklyPercent, weeklyResetsAt, err := parseUsageWindow(html, ollamaWeeklyUsageRE, "weekly")
	if err != nil {
		return QuotaSummary{}, err
	}

	summary := QuotaSummary{
		ProviderID:         "ollama",
		PlanName:           extractPlanName(html),
		SessionUsedPercent: sessionPercent,
		SessionResetsAt:    sessionResetsAt,
		WeeklyUsedPercent:  weeklyPercent,
		WeeklyResetsAt:     weeklyResetsAt,
		NotifyUsageLimits:  ollamaNotifyUsageRE.MatchString(html),
		State:              QuotaStateLive,
		Source:             QuotaSourceOllamaSettingsHTML,
		FetchedAt:          fetchedAt.UTC(),
		ExpiresAt:          expiresAt.UTC(),
	}
	if summary.FetchedAt.IsZero() {
		summary.FetchedAt = time.Now().UTC()
	}
	if summary.ExpiresAt.IsZero() {
		summary.ExpiresAt = summary.FetchedAt
	}

	return summary, nil
}

func extractPlanName(html string) string {
	match := ollamaCloudUsagePlanRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func parseUsageWindow(html string, re *regexp.Regexp, label string) (float64, *time.Time, error) {
	match := re.FindStringSubmatch(html)
	if len(match) < 3 {
		return 0, nil, fmt.Errorf("parse %s usage block", label)
	}

	usedPercent, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	if err != nil {
		return 0, nil, fmt.Errorf("parse %s used percent: %w", label, err)
	}

	resetsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(match[2]))
	if err != nil {
		return 0, nil, fmt.Errorf("parse %s reset time: %w", label, err)
	}
	resetsAt = resetsAt.UTC()
	return usedPercent, &resetsAt, nil
}
