package web

import (
	"testing"
	"time"
)

func TestDurationFormat(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-time.Hour, "0 seconds"},
		{0, "0 seconds"},
		{time.Second, "1 second"},
		{59 * time.Second, "59 seconds"},
		{time.Minute, "1 minute"},
		{119 * time.Second, "1 minute"},
		{2 * time.Hour, "2 hours"},
		{day - time.Second, "23 hours"},
		{day, "1 day"},
		{29 * day, "29 days"},
		{30 * day, "1 month"},
		{11*month + 29*day, "11 months"},
		{year, "1 year"},
		{25 * month, "2 years"},
	}
	for _, tt := range tests {
		if got := DurationFormat(tt.d); got != tt.want {
			t.Errorf("DurationFormat(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(time.Hour), "just now"}, // clock skew
		{now, "just now"},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-time.Minute), "1 minute ago"},
		{now.Add(-5 * time.Hour), "5 hours ago"},
		{now.Add(-3 * day), "3 days ago"},
		{now.Add(-400 * day), "1 year ago"},
	}
	for _, tt := range tests {
		if got := relativeTime(tt.t, now); got != tt.want {
			t.Errorf("relativeTime(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}
