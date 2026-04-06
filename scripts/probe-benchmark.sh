#!/usr/bin/env bash
set -euo pipefail

target="${1:?usage: $0 <target> [size-bytes]}"
size_bytes="${2:-1073741824}"
remote_user="${DERPCAT_REMOTE_USER:-root}"
probe_local_bin="dist/derpcat-probe"
probe_remote_bin="dist/derpcat-probe-linux-amd64"
remote_probe="/tmp/derpcat-probe"

mkdir -p dist
go build -o "${probe_local_bin}" ./cmd/derpcat-probe
GOOS=linux GOARCH=amd64 go build -o "${probe_remote_bin}" ./cmd/derpcat-probe
scp "${probe_remote_bin}" "${remote_user}@${target}:${remote_probe}" >/dev/null
ssh "${remote_user}@${target}" "chmod 0755 '${remote_probe}'"

"./${probe_local_bin}" orchestrate --host "${target}" --user "${remote_user}" --size-bytes "${size_bytes}" --mode raw
