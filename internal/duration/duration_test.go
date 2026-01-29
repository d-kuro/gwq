package duration

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// Day notation
		{
			name:  "1 day",
			input: "1d",
			want:  24 * time.Hour,
		},
		{
			name:  "7 days",
			input: "7d",
			want:  7 * 24 * time.Hour,
		},
		{
			name:  "30 days",
			input: "30d",
			want:  30 * 24 * time.Hour,
		},

		// Standard Go durations
		{
			name:  "1 hour",
			input: "1h",
			want:  time.Hour,
		},
		{
			name:  "30 minutes",
			input: "30m",
			want:  30 * time.Minute,
		},
		{
			name:  "2 hours 30 minutes",
			input: "2h30m",
			want:  2*time.Hour + 30*time.Minute,
		},
		{
			name:  "1 second",
			input: "1s",
			want:  time.Second,
		},
		{
			name:  "500 milliseconds",
			input: "500ms",
			want:  500 * time.Millisecond,
		},

		// Whitespace handling
		{
			name:  "whitespace around duration",
			input: "  7d  ",
			want:  7 * 24 * time.Hour,
		},

		// Error cases
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "zero days",
			input:   "0d",
			wantErr: true,
		},
		{
			name:    "negative duration",
			input:   "-1h",
			wantErr: true,
		},
		{
			name:    "negative days",
			input:   "-7d",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
