package metrics

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"4h", 4 * time.Hour},
		{"90s", 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseDuration(tt.in)
			if err != nil {
				t.Fatalf("ParseDuration(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
