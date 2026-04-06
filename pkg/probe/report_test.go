package probe

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarkdownReportIncludesCoreMetrics(t *testing.T) {
	report := RunReport{
		Host:        "ktzlxc",
		Mode:        "raw",
		Direction:   "forward",
		SizeBytes:   1 << 20,
		DurationMS:  1250,
		GoodputMbps: 670.5,
		Direct:      true,
		FirstByteMS: 18,
		LossRate:    0.125,
		Retransmits: 4,
	}

	md := report.Markdown()
	for _, want := range []string{"ktzlxc", "raw", "670.5", "direct=true", "first_byte_ms=18", "loss_rate=0.125", "retransmits=4"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q: %s", want, md)
		}
	}
}

func TestRunReportJSONEncodesCoreMetrics(t *testing.T) {
	report := RunReport{
		Host:        "ktzlxc",
		Mode:        "raw",
		Direction:   "forward",
		SizeBytes:   1024,
		DurationMS:  10,
		GoodputMbps: 8.5,
		Direct:      true,
		FirstByteMS: 3,
		LossRate:    0.02,
		Retransmits: 1,
	}

	got, err := report.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, forbidden := range []string{"user", "remote_path", "listen_addr", "server_command", "client_command"} {
		if _, ok := decoded[forbidden]; ok {
			t.Fatalf("JSON unexpectedly included %q: %#v", forbidden, decoded)
		}
	}
	if decoded["host"] != "ktzlxc" || decoded["mode"] != "raw" || decoded["direct"] != true || decoded["first_byte_ms"] != float64(3) || decoded["loss_rate"] != float64(0.02) || decoded["retransmits"] != float64(1) {
		t.Fatalf("decoded report = %#v", decoded)
	}
}
