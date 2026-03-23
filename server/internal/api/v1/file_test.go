package api

import "testing"

func TestSanitizeSliceMediaFileName(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		expectOk    bool
		expectedStr string
	}{
		{name: "audio", in: "audio.m4s", expectOk: true, expectedStr: "audio.m4s"},
		{name: "video30", in: "1280x720_3004k_30_video.m4s", expectOk: true, expectedStr: "1280x720_3004k_30_video.m4s"},
		{name: "ts", in: "segment_00001.ts", expectOk: true, expectedStr: "segment_00001.ts"},
		{name: "whitespace-trim", in: " audio.m4s ", expectOk: true, expectedStr: "audio.m4s"},

		{name: "path-traversal", in: "../audio.m4s", expectOk: false},
		{name: "slash", in: "a/b.m4s", expectOk: false},
		{name: "wrong-ext", in: "audio.mkv", expectOk: false},
		{name: "wrong-case-ext", in: "audio.M4S", expectOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeSliceMediaFileName(tt.in)
			if tt.expectOk {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.expectedStr {
					t.Fatalf("result = %q, want %q", got, tt.expectedStr)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil (result=%q)", got)
			}
		})
	}
}

