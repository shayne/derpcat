package derphole

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressReporterUsesRecentSmoothedRate(t *testing.T) {
	start := time.Unix(0, 0)
	now := start
	prevProgressNow := progressNow
	progressNow = func() time.Time { return now }
	t.Cleanup(func() { progressNow = prevProgressNow })

	var out bytes.Buffer
	progress := NewProgressReporter(&out, 20*1024*1024)

	now = start.Add(100 * time.Millisecond)
	progress.Add(10 * 1024 * 1024)

	now = start.Add(10 * time.Second)
	progress.Add(1024)

	got := out.String()
	if !strings.Contains(got, "719.1KiB/s") {
		t.Fatalf("progress output = %q, want tqdm-style smoothed recent rate near 719.1KiB/s", got)
	}
	if strings.Contains(got, "1.0MiB/s") {
		t.Fatalf("progress output = %q, want tqdm-style recent rate, not cumulative average", got)
	}
}

func TestProgressReporterClearsStaleTrailingCharacters(t *testing.T) {
	start := time.Unix(0, 0)
	now := start
	prevProgressNow := progressNow
	progressNow = func() time.Time { return now }
	t.Cleanup(func() { progressNow = prevProgressNow })

	var out bytes.Buffer
	progress := NewProgressReporter(&out, 1000*1024*1024)
	progress.lastRender = start.Add(-time.Second)

	now = start.Add(time.Nanosecond)
	progress.Add(100 * 1024 * 1024)

	now = start.Add(51 * time.Second)
	progress.Finish()

	raw := out.String()
	finalLine := strings.TrimRight(lastRawProgressLine(raw), " ")
	visibleLine := strings.TrimRight(renderedTerminalLine(raw), " ")
	if visibleLine != finalLine {
		t.Fatalf("visible progress line = %q, want %q; raw output = %q", visibleLine, finalLine, raw)
	}
}

func lastRawProgressLine(output string) string {
	index := strings.LastIndex(output, "\r")
	if index < 0 {
		return strings.TrimSuffix(output, "\n")
	}
	return strings.TrimSuffix(output[index+1:], "\n")
}

func renderedTerminalLine(output string) string {
	var line []rune
	var completed []rune
	column := 0
	for _, r := range output {
		switch r {
		case '\r':
			column = 0
		case '\n':
			completed = append(completed[:0], line...)
			line = line[:0]
			column = 0
		default:
			for len(line) < column {
				line = append(line, ' ')
			}
			if column == len(line) {
				line = append(line, r)
			} else {
				line[column] = r
			}
			column++
		}
	}
	if len(line) > 0 {
		completed = line
	}
	return string(completed)
}
