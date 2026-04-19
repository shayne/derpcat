package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDerpholeWebLandingKeepsDemoInTechnicalFrontDoor(t *testing.T) {
	t.Parallel()

	htmlPath := filepath.Join("..", "web", "derphole", "index.html")
	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read web landing page: %v", err)
	}
	html := string(data)

	required := []string{
		`<main class="site-shell">`,
		`<section id="top" class="intro"`,
		`<section id="demo" class="demo-grid"`,
		`href="https://github.com/shayne/derphole"`,
		`npx -y derphole@latest`,
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Fatalf("web landing page missing %q", want)
		}
	}

	demoIDs := []string{
		`id="select-send-file"`,
		`id="start-send"`,
		`id="send-token"`,
		`id="copy-token"`,
		`id="receive-token"`,
		`id="start-receive"`,
		`id="send-progress"`,
		`id="receive-progress"`,
	}
	for _, id := range demoIDs {
		if !strings.Contains(html, id) {
			t.Fatalf("web demo lost required element %s", id)
		}
	}
}

func TestDerpholeWebStylesAvoidDecorativeLandingPagePatterns(t *testing.T) {
	t.Parallel()

	cssPath := filepath.Join("..", "web", "derphole", "styles.css")
	data, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read web styles: %v", err)
	}
	css := string(data)

	required := []string{
		"oklch(",
		"color-scheme: light dark",
		"--space-xs:",
		"font-variant-numeric: tabular-nums",
		"@media (prefers-color-scheme: dark)",
		"@media (prefers-reduced-motion: reduce)",
	}
	for _, want := range required {
		if !strings.Contains(css, want) {
			t.Fatalf("web styles missing %q", want)
		}
	}

	forbidden := []string{
		"background-clip: text",
		"-webkit-background-clip: text",
		"radial-gradient",
		"#fbebcd",
		"#f4ead9",
	}
	for _, bad := range forbidden {
		if strings.Contains(css, bad) {
			t.Fatalf("web styles still contain decorative pattern %q", bad)
		}
	}
}
