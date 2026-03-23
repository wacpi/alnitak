package service

import "testing"

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{name: "zero", in: 0, want: "PT0H0M0.000S"},
		{name: "61s", in: 61, want: "PT0H1M1.000S"},
		{name: "3661.5s", in: 3661.5, want: "PT1H1M1.500S"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.in)
			if got != tt.want {
				t.Fatalf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseRangeInt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int64
	}{
		{name: "start", in: "0", want: 0},
		{name: "end", in: "834", want: 834},
		{name: "end2", in: "5151", want: 5151},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRangeInt(tt.in)
			if got != tt.want {
				t.Fatalf("parseRangeInt(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

