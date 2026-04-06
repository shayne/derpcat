#!/usr/bin/env bash
set -euo pipefail

size_bytes_values=(10240 1048576 10485760 52428800 134217728 1073741824)

for host in ktzlxc canlxc uklxc orange-india.exe.xyz; do
  for size_bytes in "${size_bytes_values[@]}"; do
    ./scripts/probe-benchmark.sh "${host}" "${size_bytes}"
  done
done
