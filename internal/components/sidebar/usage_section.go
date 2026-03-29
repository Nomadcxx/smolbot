package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type UsageSection struct {
	summary *client.UsageSummary
}

func (s UsageSection) Title() string { return "USAGE" }

func (s UsageSection) ItemCount() int { return 0 }

func (s UsageSection) Render(width, _ int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}
	if s.summary == nil {
		return styleLine("—", width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	}

	provider := strings.TrimSpace(s.summary.ProviderID)
	model := strings.TrimSpace(s.summary.ModelName)
	label := usageLabel(provider, model)

	lines := []string{
		renderValue(label, width, t, func(th *theme.Theme) color.Color { return th.Secondary }),
		styleLine("Observed", width, t, func(th *theme.Theme) color.Color { return th.Accent }),
		styleLine(fmt.Sprintf("session %s", formatTokens(s.summary.SessionTokens)), width, t, func(th *theme.Theme) color.Color { return th.Text }),
		styleLine(fmt.Sprintf("today %s", formatTokens(s.summary.TodayTokens)), width, t, func(th *theme.Theme) color.Color { return th.TextMuted }),
		styleLine(fmt.Sprintf("week %s", formatTokens(s.summary.WeeklyTokens)), width, t, func(th *theme.Theme) color.Color { return th.TextMuted }),
	}
	if requests := usageRequestLine(s.summary); requests != "" {
		lines = append(lines, styleLine(requests, width, t, func(th *theme.Theme) color.Color { return th.TextMuted }))
	}

	if quotaLines := usageQuotaLines(s.summary, width, t); len(quotaLines) > 0 {
		lines = append(lines, quotaLines...)
	}
	if badge := usageWarningLabel(s.summary); badge != "" {
		lines = append(lines, styleLine(badge, width, t, usageWarningColor))
	}
	return joinNonEmpty(lines...)
}

func usageLabel(provider, model string) string {
	switch {
	case provider != "" && model != "":
		if strings.HasPrefix(model, provider+"/") {
			return model
		}
		return provider + " / " + model
	case provider != "":
		return provider
	case model != "":
		return model
	default:
		return "—"
	}
}

func usageWarningLabel(summary *client.UsageSummary) string {
	if summary == nil {
		return ""
	}
	status := strings.TrimSpace(summary.BudgetStatus)
	level := strings.TrimSpace(summary.WarningLevel)
	switch {
	case status != "" && level != "":
		return status + " " + level
	case status != "":
		return status
	case level != "":
		return level
	default:
		return ""
	}
}

func usageWarningColor(t *theme.Theme) color.Color {
	if t == nil {
		return nil
	}
	return t.Warning
}

func usageQuotaLines(summary *client.UsageSummary, width int, t *theme.Theme) []string {
	if summary == nil || summary.Quota == nil {
		return nil
	}

	quota := summary.Quota
	lines := []string{
		styleLine("Quota", width, t, func(th *theme.Theme) color.Color { return th.Accent }),
	}
	switch strings.ToLower(strings.TrimSpace(quota.State)) {
	case "expired", "unavailable":
		lines = append(lines, styleLine("quota "+strings.ToLower(strings.TrimSpace(quota.State)), width, t, usageWarningColor))
		return lines
	case "live", "":
	default:
		lines = append(lines, styleLine("quota "+strings.ToLower(strings.TrimSpace(quota.State)), width, t, usageWarningColor))
		return lines
	}

	if plan := strings.TrimSpace(quota.PlanName); plan != "" {
		lines = append(lines, styleLine(plan, width, t, func(th *theme.Theme) color.Color { return th.Text }))
	}
	lines = append(lines, styleLine(fmt.Sprintf("session %.1f%%", quota.SessionUsedPercent), width, t, severityColor(quota.SessionUsedPercent)))
	lines = append(lines, styleLine(fmt.Sprintf("week %.1f%%", quota.WeeklyUsedPercent), width, t, severityColor(quota.WeeklyUsedPercent)))
	return lines
}

func severityColor(pct float64) func(*theme.Theme) color.Color {
	return func(th *theme.Theme) color.Color {
		if th == nil {
			return nil
		}
		switch {
		case pct >= 80:
			return th.Error
		case pct >= 60:
			return th.Warning
		default:
			return th.TextMuted
		}
	}
}

func usageRequestLine(summary *client.UsageSummary) string {
	if summary == nil {
		return ""
	}
	if summary.SessionRequests == 0 && summary.TodayRequests == 0 && summary.WeeklyRequests == 0 {
		return ""
	}
	return fmt.Sprintf("reqs %d/%d/%d", summary.SessionRequests, summary.TodayRequests, summary.WeeklyRequests)
}
