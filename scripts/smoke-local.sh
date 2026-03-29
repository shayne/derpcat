#!/usr/bin/env bash
set -euo pipefail

tmp="$(mktemp -d)"
cleanup() {
  if [[ -n "${listener_pid:-}" ]]; then
    kill "${listener_pid}" 2>/dev/null || true
    wait "${listener_pid}" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

mise run build

listener_log="$tmp/listener.log"
sender_log="$tmp/sender.log"
output_file="$tmp/output.txt"

dist/derpcat listen >"$output_file" 2>"$listener_log" &
listener_pid=$!

token=""
for _ in $(seq 1 100); do
  token="$(grep -E '^[A-Za-z0-9_-]{20,}$' "$listener_log" | head -n 1 || true)"
  if [[ -n "$token" ]]; then
    break
  fi
  sleep 0.1
done

if [[ -z "$token" ]]; then
  echo "failed to capture listener token" >&2
  exit 1
fi

printf 'hello smoke' | dist/derpcat send "$token" >"$tmp/sender.out" 2>"$sender_log"
wait "$listener_pid"

test "$(cat "$output_file")" = "hello smoke"
