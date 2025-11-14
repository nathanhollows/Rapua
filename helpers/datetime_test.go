package helpers_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/helpers"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name string
		date string
		want time.Time
	}{
		{
			name: "Valid Date",
			date: "2021-01-01",
			want: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Invalid Date",
			date: "2021-01-32",
			want: time.Time{},
		},
		{
			name: "Invalid Format",
			date: "01-01-2021",
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helpers.ParseDate(tt.date)
			if !got.Equal(tt.want) {
				t.Errorf("ParseDate() = %v, want %v", got, tt.want)
			}
			if err != nil && !got.IsZero() {
				t.Errorf("ParseDate() error = %v, want nil", err)
			}
		})
	}
}

// TestHelpers_ParseTime tests the ParseTime function.
func TestParseTime(t *testing.T) {
	tests := []struct {
		name string
		time string
		want time.Time
	}{
		{
			name: "Valid Time",
			time: "15:04:05",
			want: time.Date(0, 1, 1, 15, 4, 5, 0, time.UTC),
		},
		{
			name: "Short Time",
			time: "15:04",
			want: time.Date(0, 1, 1, 15, 4, 0, 0, time.UTC),
		},
		{
			name: "Shortest Time",
			time: "15",
			want: time.Date(0, 1, 1, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "Invalid Time",
			time: "25:00:00",
			want: time.Time{},
		},
		{
			name: "Invalid Format",
			time: "3:04:05 PM",
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helpers.ParseTime(tt.time)
			if !got.Equal(tt.want) {
				t.Errorf("ParseTime() = %v, want %v", got, tt.want)
			}
			if err != nil && !got.IsZero() {
				t.Errorf("ParseTime() error = %v, want nil", err)
			}
		})
	}
}

// TestHelpers_ParseDateTime tests the ParseDateTime function.
func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name string
		date string
		time string
		want time.Time
	}{
		{
			name: "Valid Date and Time",
			date: "2021-01-01",
			time: "15:04:05",
			want: time.Date(2021, 1, 1, 15, 4, 5, 0, time.UTC),
		},
		{
			name: "Invalid Date",
			date: "2021-01-32",
			time: "15:04:05",
			want: time.Time{},
		},
		{
			name: "Invalid Time",
			date: "2021-01-01",
			time: "25:00:00",
			want: time.Time{},
		},
		{
			name: "Invalid Format",
			date: "01-01-2021",
			time: "3:04:05 PM",
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helpers.ParseDateTime(tt.date, tt.time)
			if !got.Equal(tt.want) {
				t.Errorf("ParseDateTime() = %v, want %v", got, tt.want)
			}
			if err != nil && !got.IsZero() {
				t.Errorf("ParseDateTime() error = %v, want nil", err)
			}
		})
	}
}
