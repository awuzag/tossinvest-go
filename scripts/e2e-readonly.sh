#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${TOSSINVEST_ENV_FILE:-.env.toss}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}" >&2
  exit 2
fi

echo "env file: ${ENV_FILE}"
echo "env keys:"
awk -F= '/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*=/{gsub(/^[[:space:]]+|[[:space:]]+$/, "", $1); print "  " $1}' "${ENV_FILE}"

if command -v curl >/dev/null 2>&1; then
  ip="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [[ -n "${ip}" ]]; then
    echo "current public ip: ${ip}"
  else
    echo "current public ip: unavailable"
  fi
fi

set +e
output="$(
  TOSSINVEST_E2E=1 \
  TOSSINVEST_ENV_FILE="${ENV_FILE}" \
  go test -tags=e2e -run TestE2EReadOnlySmoke -count=1 -v . 2>&1
)"
status=$?
set -e

printf '%s\n' "${output}"

if grep -q "IP address not allowed" <<<"${output}"; then
  echo "read-only live e2e reached Toss Invest, but the current IP is not allowlisted." >&2
  echo "Add the current public IP to the Toss Invest app allowlist, then rerun this script." >&2
  exit 2
fi

if grep -q "credentials are not configured" <<<"${output}"; then
  echo "read-only live e2e did not run because credentials were not configured." >&2
  exit 2
fi

exit "${status}"
