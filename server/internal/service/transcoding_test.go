package service

import "testing"

func TestAvc1CodecStringFromH264ProfileLevel(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		level   int
		want    string
	}{
		{name: "High-4.2", profile: "High", level: 42, want: "avc1.64002A"},
		{name: "High-4.0", profile: "High", level: 40, want: "avc1.640028"},
		{name: "Main-4.2", profile: "Main", level: 42, want: "avc1.4D002A"},
		{name: "Baseline-3.1", profile: "baseline", level: 31, want: "avc1.42001F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := avc1CodecStringFromH264ProfileLevel(tt.profile, tt.level)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("codec string = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("unsupported-profile", func(t *testing.T) {
		_, err := avc1CodecStringFromH264ProfileLevel("Unknown", 42)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

