#!/usr/bin/env bash
set -euo pipefail

target="${1:?usage: $0 <hetz|pve1>}"
tmp="$(mktemp -d)"
remote_base="/tmp/derpcat-smoke-$$"
local_listener_pid=""

remote() {
  ssh "root@${target}" "bash -lc $(printf '%q' "$1")"
}

cleanup() {
  if [[ -n "${local_listener_pid}" ]]; then
    kill "${local_listener_pid}" 2>/dev/null || true
    wait "${local_listener_pid}" 2>/dev/null || true
  fi
  remote "if [[ -f '${remote_base}.pid' ]]; then kill \$(cat '${remote_base}.pid') 2>/dev/null || true; fi; rm -f '${remote_base}.pid' '${remote_base}.out' '${remote_base}.err' '${remote_base}.sender.out' '${remote_base}.sender.err'" >/dev/null 2>&1 || true
  rm -rf "${tmp}"
}
trap cleanup EXIT

wait_for_remote_token() {
  local token=""
  for _ in $(seq 1 100); do
    token="$(remote "grep -E '^[A-Za-z0-9_-]{20,}\$' '${remote_base}.err' | head -n 1 || true")"
    if [[ -n "${token}" ]]; then
      printf '%s\n' "${token}"
      return 0
    fi
    sleep 0.1
  done
  return 1
}

wait_for_local_token() {
  local log_file="$1"
  local token=""
  for _ in $(seq 1 100); do
    token="$(grep -E '^[A-Za-z0-9_-]{20,}$' "${log_file}" | head -n 1 || true)"
    if [[ -n "${token}" ]]; then
      printf '%s\n' "${token}"
      return 0
    fi
    sleep 0.1
  done
  return 1
}

wait_for_remote_exit() {
  for _ in $(seq 1 120); do
    if ! remote "if [[ -f '${remote_base}.pid' ]]; then kill -0 \$(cat '${remote_base}.pid') 2>/dev/null; else false; fi" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

wait_for_local_exit() {
  local pid="$1"
  for _ in $(seq 1 120); do
    if ! kill -0 "${pid}" 2>/dev/null; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

dump_remote_logs() {
  remote "echo '--- remote err'; sed -n '1,160p' '${remote_base}.err' 2>/dev/null || true; echo '--- remote out'; sed -n '1,160p' '${remote_base}.out' 2>/dev/null || true; echo '--- remote sender err'; sed -n '1,160p' '${remote_base}.sender.err' 2>/dev/null || true; echo '--- remote sender out'; sed -n '1,160p' '${remote_base}.sender.out' 2>/dev/null || true"
}

mise run build
mise run build-linux-amd64
scp dist/derpcat-linux-amd64 "root@${target}:/usr/local/bin/derpcat"
remote "chmod +x /usr/local/bin/derpcat && /usr/local/bin/derpcat --help >/dev/null 2>&1"

payload_local_to_remote="hello local-to-${target}-$(date +%s)"
remote "rm -f '${remote_base}.pid' '${remote_base}.out' '${remote_base}.err'; nohup /usr/local/bin/derpcat listen >'${remote_base}.out' 2>'${remote_base}.err' </dev/null & echo \$! > '${remote_base}.pid'"
remote_token="$(wait_for_remote_token)" || {
  echo "failed to capture remote listener token" >&2
  dump_remote_logs >&2
  exit 1
}
printf '%s' "${payload_local_to_remote}" | dist/derpcat send "${remote_token}" >"${tmp}/local-sender.out" 2>"${tmp}/local-sender.err"
wait_for_remote_exit || {
  echo "remote listener did not exit" >&2
  dump_remote_logs >&2
  exit 1
}
remote_output="$(remote "cat '${remote_base}.out'")"
if [[ "${remote_output}" != "${payload_local_to_remote}" ]]; then
  echo "remote output mismatch" >&2
  printf 'want=%q\n' "${payload_local_to_remote}" >&2
  printf ' got=%q\n' "${remote_output}" >&2
  dump_remote_logs >&2
  exit 1
fi

payload_remote_to_local="hello ${target}-to-local-$(date +%s)"
local_listener_log="${tmp}/local-listener.err"
local_listener_out="${tmp}/local-listener.out"
dist/derpcat listen >"${local_listener_out}" 2>"${local_listener_log}" &
local_listener_pid=$!
local_token="$(wait_for_local_token "${local_listener_log}")" || {
  echo "failed to capture local listener token" >&2
  sed -n '1,160p' "${local_listener_log}" >&2 || true
  exit 1
}
remote "printf '%s' '${payload_remote_to_local}' | /usr/local/bin/derpcat send '${local_token}' >'${remote_base}.sender.out' 2>'${remote_base}.sender.err'"
wait_for_local_exit "${local_listener_pid}" || {
  echo "local listener did not exit" >&2
  sed -n '1,160p' "${local_listener_log}" >&2 || true
  dump_remote_logs >&2
  exit 1
}
local_listener_pid=""
local_output="$(cat "${local_listener_out}")"
if [[ "${local_output}" != "${payload_remote_to_local}" ]]; then
  echo "local output mismatch" >&2
  printf 'want=%q\n' "${payload_remote_to_local}" >&2
  printf ' got=%q\n' "${local_output}" >&2
  sed -n '1,160p' "${local_listener_log}" >&2 || true
  dump_remote_logs >&2
  exit 1
fi
