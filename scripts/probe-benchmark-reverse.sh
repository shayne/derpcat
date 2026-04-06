#!/usr/bin/env bash
set -euo pipefail

target="${1:?usage: $0 <target> [size-bytes]}"
size_bytes="${2:-1073741824}"
remote_user="${DERPCAT_REMOTE_USER:-root}"
probe_remote_bin="dist/derpcat-probe-linux-amd64"
remote_probe="/tmp/derpcat-probe"
peer_host="${DERPCAT_PROBE_PEER_HOST:?set DERPCAT_PROBE_PEER_HOST to a reachable peer address for reverse mode}"
peer_user="${DERPCAT_PROBE_PEER_USER:-${USER:-$(id -un)}}"

mkdir -p dist
GOOS=linux GOARCH=amd64 go build -o "${probe_remote_bin}" ./cmd/derpcat-probe
scp "${probe_remote_bin}" "${remote_user}@${target}:${remote_probe}" >/dev/null
ssh "${remote_user}@${target}" "chmod 0755 '${remote_probe}'"

ssh "${remote_user}@${target}" "/tmp/derpcat-probe orchestrate --host '${peer_host}' --user '${peer_user}' --size-bytes '${size_bytes}' --mode raw"
