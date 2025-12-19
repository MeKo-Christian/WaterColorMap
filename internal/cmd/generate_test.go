package cmd

import (
	"testing"
)

func TestParseBBox(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [4]float64
		wantErr bool
	}{
		{
			name:    "valid bbox",
			input:   "9.7,52.3,9.9,52.4",
			want:    [4]float64{9.7, 52.3, 9.9, 52.4},
			wantErr: false,
		},
		{
			name:    "valid bbox with spaces",
			input:   "9.7, 52.3, 9.9, 52.4",
			want:    [4]float64{9.7, 52.3, 9.9, 52.4},
			wantErr: false,
		},
		{
			name:    "negative coordinates",
			input:   "-122.5,37.7,-122.3,37.9",
			want:    [4]float64{-122.5, 37.7, -122.3, 37.9},
			wantErr: false,
		},
		{
			name:    "too few values",
			input:   "9.7,52.3,9.9",
			wantErr: true,
		},
		{
			name:    "too many values",
			input:   "9.7,52.3,9.9,52.4,10.0",
			wantErr: true,
		},
		{
			name:    "invalid number",
			input:   "abc,52.3,9.9,52.4",
			wantErr: true,
		},
		{
			name:    "minLon >= maxLon",
			input:   "10.0,52.3,9.9,52.4",
			wantErr: true,
		},
		{
			name:    "minLat >= maxLat",
			input:   "9.7,52.5,9.9,52.4",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBBox(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBBox(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseBBox(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseBBox(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
