package service

import (
	"testing"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1973-12-15", "1973-12-15"},
		{"1973-12-15T00:00:00Z", "1973-12-15"},
		{"1973-12-15 00:00:00", "1973-12-15"},
		{"2026-03-22T08:03:44Z", "2026-03-22"},
		{"short", "short"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatDate(tt.input)
			if got != tt.expected {
				t.Errorf("formatDate(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-03-22T08:03:44Z", "2026-03-22 08:03:44"},
		{"2026-03-22 08:03:44", "2026-03-22 08:03:44"},
		{"2026-03-22T10:00:00+02:00", "2026-03-22 10:00:00"},
		{"short", "short"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatDateTime(tt.input)
			if got != tt.expected {
				t.Errorf("formatDateTime(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
