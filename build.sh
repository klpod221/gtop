#!/usr/bin/env bash
set -euo pipefail

BINARY="gtop"
CAPS="cap_perfmon,cap_dac_read_search=ep"

echo ":: Building ${BINARY}..."
go build -o "${BINARY}" .

echo ":: Setting capabilities (${CAPS})..."
sudo setcap "${CAPS}" "./${BINARY}"
