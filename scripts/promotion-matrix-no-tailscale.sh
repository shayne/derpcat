#!/usr/bin/env bash
set -euo pipefail

size_mib="${1:-1024}"
tmp="$(mktemp -d)"
hosts=(hetz ktzlxc pve1)

dump_logs() {
  for file in "${tmp}"/*.log; do
    [[ -f "${file}" ]] || continue
    echo "--- ${file}" >&2
    sed -n '1,220p' "${file}" >&2 || true
  done
}

cleanup() {
  rm -rf "${tmp}"
}

assert_no_tailscale_candidates() {
  local log_file="$1"

  if grep -Eq '(^|[^0-9])100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[0-9]+\.[0-9]+(:[0-9]+)?([^0-9]|$)|fd7a:115c:a1e0:' "${log_file}"; then
    echo "tailscale candidate leaked into ${log_file}" >&2
    exit 1
  fi
}

trap 'rc=$?; if [[ ${rc} -ne 0 ]]; then dump_logs; fi; cleanup; exit ${rc}' EXIT

for host in "${hosts[@]}"; do
  forward_log="${tmp}/${host}-forward.log"
  reverse_log="${tmp}/${host}-reverse.log"

  echo "running no-tailscale forward promotion test host=${host} size_mib=${size_mib}"
  DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/promotion-test.sh "${host}" "${size_mib}" | tee "${forward_log}"
  assert_no_tailscale_candidates "${forward_log}"

  echo "running no-tailscale reverse promotion test host=${host} size_mib=${size_mib}"
  DERPCAT_TEST_DISABLE_TAILSCALE_CANDIDATES=1 ./scripts/promotion-test-reverse.sh "${host}" "${size_mib}" | tee "${reverse_log}"
  assert_no_tailscale_candidates "${reverse_log}"
done
