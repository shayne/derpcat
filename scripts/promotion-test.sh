#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DERPCAT_BENCH_TOOL=derpcat DERPCAT_BENCH_DIRECTION=forward exec "${script_dir}/promotion-benchmark-driver.sh" "$@"
