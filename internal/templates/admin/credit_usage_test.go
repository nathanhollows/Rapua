package templates_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/stretchr/testify/assert"
)

func TestCreditUsageChart_SaysSoWhenThePeriodHasNoUsage(t *testing.T) {
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	zeros := []services.RunStartSummary{{Date: day}, {Date: day.AddDate(0, 0, 1)}}

	for _, period := range []string{"week", "month", "year"} {
		html := render(t, templates.CreditUsageChart(zeros, period))
		assert.Contains(t, html, "No credits used in this period.", period)
		assert.NotContains(t, html, "charts-css", period)
	}
	assert.Contains(t, render(t, templates.CreditUsageChart(nil, "week")), "No credits used in this period.")
}

func TestCreditUsageChart_DrawsTheChartWhenThereIsUsage(t *testing.T) {
	day := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	html := render(t, templates.CreditUsageChart([]services.RunStartSummary{{Date: day, Count: 3, Ratio: 1}}, "week"))

	assert.Contains(t, html, "charts-css")
	assert.Contains(t, html, "3 credits used")
	assert.NotContains(t, html, "No credits used")
}

func TestSettingsCreditUsage_PreselectsThePeriodItWasGiven(t *testing.T) {
	html := render(t, templates.SettingsCreditUsage(1, 2, 3, nil, nil, "month"))

	assert.Regexp(t, `value="month"[^>]*checked`, html)
	assert.NotRegexp(t, `value="week"[^>]*checked`, html)
}
