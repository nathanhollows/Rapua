package models_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

func TestInstance_GetStatus(t *testing.T) {
	tests := []struct {
		name      string
		startTime bun.NullTime
		endTime   bun.NullTime
		want      models.GameStatus
	}{
		{
			name:      "Closed - No Times",
			startTime: bun.NullTime{},
			endTime:   bun.NullTime{},
			want:      models.Closed,
		},
		{
			name:      "Closed - End Time Only",
			startTime: bun.NullTime{},
			endTime:   bun.NullTime{Time: time.Now().Add(-time.Hour)},
			want:      models.Closed,
		},
		{
			name:      "Closed - End Time in Past",
			startTime: bun.NullTime{Time: time.Now().Add(-2 * time.Hour)},
			endTime:   bun.NullTime{Time: time.Now().Add(-time.Hour)},
			want:      models.Closed,
		},
		{
			name:      "Scheduled - Start Time Only",
			startTime: bun.NullTime{Time: time.Now().Add(time.Hour)},
			endTime:   bun.NullTime{},
			want:      models.Scheduled,
		},
		{
			name:      "Scheduled - Start Time in Future",
			startTime: bun.NullTime{Time: time.Now().Add(time.Hour)},
			endTime:   bun.NullTime{Time: time.Now().Add(2 * time.Hour)},
			want:      models.Scheduled,
		},
		{
			name:      "Active - Start Time in Past, End Time in Future",
			startTime: bun.NullTime{Time: time.Now().Add(-time.Hour)},
			endTime:   bun.NullTime{Time: time.Now().Add(time.Hour)},
			want:      models.Active,
		},
		{
			name:      "Active - Start Time in Past, No End Time",
			startTime: bun.NullTime{Time: time.Now().Add(-time.Hour)},
			endTime:   bun.NullTime{},
			want:      models.Active,
		},
		{
			name:      "Closed - No Start Time, End Time in Past",
			startTime: bun.NullTime{},
			endTime:   bun.NullTime{Time: time.Now().Add(-time.Hour)},
			want:      models.Closed,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				instance := &models.Instance{
					StartTime: tt.startTime,
					EndTime:   tt.endTime,
				}

				if got := instance.GetStatus(); got != tt.want {
					t.Errorf("Instance.GetStatus() = %v, want %v", got, tt.want)
				}
			},
		)
	}
}
