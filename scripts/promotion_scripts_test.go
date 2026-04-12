package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromotionWrappersUseSharedDriver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		file      string
		tool      string
		direction string
	}{
		{
			name:      "derpcat forward",
			file:      "promotion-test.sh",
			tool:      "derpcat",
			direction: "forward",
		},
		{
			name:      "derpcat reverse",
			file:      "promotion-test-reverse.sh",
			tool:      "derpcat",
			direction: "reverse",
		},
		{
			name:      "derphole forward",
			file:      "derphole-promotion-test.sh",
			tool:      "derphole",
			direction: "forward",
		},
		{
			name:      "derphole reverse",
			file:      "derphole-promotion-test-reverse.sh",
			tool:      "derphole",
			direction: "reverse",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(".", tc.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.file, err)
			}
			body := string(data)

			if !strings.Contains(body, `promotion-benchmark-driver.sh`) {
				t.Fatalf("%s does not invoke the shared benchmark driver", tc.file)
			}
			if !strings.Contains(body, `DERPCAT_BENCH_TOOL=`+tc.tool) {
				t.Fatalf("%s does not declare DERPCAT_BENCH_TOOL=%s", tc.file, tc.tool)
			}
			if !strings.Contains(body, `DERPCAT_BENCH_DIRECTION=`+tc.direction) {
				t.Fatalf("%s does not declare DERPCAT_BENCH_DIRECTION=%s", tc.file, tc.direction)
			}
		})
	}
}
