package worker

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgress_Update(t *testing.T) {
	p := NewProgress(10, false)

	p.Update(5, 10, 0)

	if p.completed != 5 {
		t.Errorf("Expected completed=5, got %d", p.completed)
	}
	if p.total != 10 {
		t.Errorf("Expected total=10, got %d", p.total)
	}
}

func TestProgress_Print(t *testing.T) {
	var buf bytes.Buffer

	p := NewProgress(10, true)
	p.output = &buf
	p.startTime = time.Now().Add(-10 * time.Second) // Simulate 10 seconds elapsed

	p.Update(5, 10, 1)

	output := buf.String()

	// Should contain progress bar
	if !strings.Contains(output, "█") {
		t.Error("Expected progress bar in output")
	}

	// Should show completed/total
	if !strings.Contains(output, "5/10 tiles") {
		t.Errorf("Expected '5/10 tiles' in output, got: %s", output)
	}

	// Should show failed count
	if !strings.Contains(output, "(1 failed)") {
		t.Errorf("Expected '(1 failed)' in output, got: %s", output)
	}

	// Should show rate
	if !strings.Contains(output, "tiles/sec") {
		t.Errorf("Expected 'tiles/sec' in output, got: %s", output)
	}

	// Should show ETA (since not complete)
	if !strings.Contains(output, "ETA:") {
		t.Errorf("Expected 'ETA:' in output, got: %s", output)
	}
}

func TestProgress_Done(t *testing.T) {
	var buf bytes.Buffer

	p := NewProgress(3, true)
	p.output = &buf
	p.startTime = time.Now().Add(-3 * time.Second)

	p.Update(3, 3, 0)
	buf.Reset() // Clear previous output

	p.Done()

	output := buf.String()

	// Should show "Done" message
	if !strings.Contains(output, "Done in") {
		t.Errorf("Expected 'Done in' in output, got: %s", output)
	}

	// Should end with newline
	if !strings.HasSuffix(output, "\n") {
		t.Error("Expected output to end with newline")
	}
}

func TestProgress_Summary(t *testing.T) {
	p := NewProgress(10, false)
	p.startTime = time.Now().Add(-10 * time.Second)

	p.Update(10, 10, 2)

	summary := p.Summary()

	if !strings.Contains(summary, "8/10 tiles") {
		t.Errorf("Expected '8/10 tiles' (successful) in summary, got: %s", summary)
	}

	if !strings.Contains(summary, "2 failed") {
		t.Errorf("Expected '2 failed' in summary, got: %s", summary)
	}
}

func TestProgress_Disabled(t *testing.T) {
	var buf bytes.Buffer

	p := NewProgress(10, false) // Disabled
	p.output = &buf

	p.Update(5, 10, 0)

	// Should not produce output when disabled
	if buf.Len() != 0 {
		t.Errorf("Expected no output when disabled, got: %s", buf.String())
	}
}

func TestProgress_Callback(t *testing.T) {
	p := NewProgress(10, false)

	callback := p.Callback()

	callback(5, 10, 1)

	if p.completed != 5 {
		t.Errorf("Expected completed=5, got %d", p.completed)
	}
	if p.failed != 1 {
		t.Errorf("Expected failed=1, got %d", p.failed)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		expected string
		duration time.Duration
	}{
		{duration: 30 * time.Second, expected: "30s"},
		{duration: 90 * time.Second, expected: "1m30s"},
		{duration: 5 * time.Minute, expected: "5m0s"},
		{duration: 65 * time.Minute, expected: "1h5m"},
		{duration: 2*time.Hour + 30*time.Minute, expected: "2h30m"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}
