package admin //nolint:testpackage // The Handler's fields are unexported.

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Answers each summary request with the next canned result: week, month, year.
type usageByCall struct {
	results [][]services.RunStartSummary
	calls   int
}

func (u *usageByCall) GetCreditAdjustments(context.Context, services.CreditAdjustmentFilter) ([]models.CreditAdjustments, error) {
	return nil, nil
}

func (u *usageByCall) GetRunStartLogsSummary(context.Context, services.RunStartLogFilter) ([]services.RunStartSummary, error) {
	res := u.results[u.calls]
	u.calls++
	return res, nil
}

func used(n int) []services.RunStartSummary { return []services.RunStartSummary{{Count: n}} }
func quiet() []services.RunStartSummary     { return []services.RunStartSummary{{Count: 0}, {Count: 0}} }

func TestDefaultCreditUsage_OpensOnTheFirstPeriodWithUsage(t *testing.T) {
	cases := []struct {
		name    string
		results [][]services.RunStartSummary
		want    string
	}{
		{"week has usage", [][]services.RunStartSummary{used(2), used(5), used(9)}, week},
		{"only the month has usage", [][]services.RunStartSummary{quiet(), used(5), used(9)}, month},
		{"only the year has usage", [][]services.RunStartSummary{quiet(), quiet(), used(9)}, year},
		{"no usage anywhere stays on the week", [][]services.RunStartSummary{quiet(), quiet(), quiet()}, week},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{creditService: &usageByCall{results: tc.results}}

			period, usage, err := h.defaultCreditUsage(context.Background(), "user")

			require.NoError(t, err)
			assert.Equal(t, tc.want, period)
			assert.NotEmpty(t, usage)
		})
	}
}

func TestCreditUsage_RejectsAnUnknownPeriod(t *testing.T) {
	h := &Handler{creditService: &usageByCall{}}

	_, err := h.creditUsage(context.Background(), "user", "decade")

	assert.Error(t, err)
}
