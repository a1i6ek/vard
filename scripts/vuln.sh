#!/usr/bin/env bash
#
# Report the known vulnerabilities the Go code reaches, as govulncheck does,
# and fail on any of them but the advisories VULN_IGNORE lists (space
# separated; the Makefile says why each is there). govulncheck itself has no
# way to leave one out.
#
#   VULN_IGNORE="GO-2026-6909" scripts/vuln.sh ./internal/...

set -euo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

need go "https://go.dev/dl"
need jq "https://jqlang.org"

cd "$(dirname "${BASH_SOURCE[0]}")/.."

report="$(go tool govulncheck -format json "$@")"

# A finding whose trace names a function is one the code calls; the others
# are only in a module or package it imports, which govulncheck reports and
# does not fail on either.
called="$(jq -r 'select(.finding != null and .finding.trace[0].function != null) | .finding.osv' <<<"$report" | sort -u)"
summary() { jq -r --arg id "$1" 'select(.osv != null and .osv.id == $id) | .osv.summary' <<<"$report" | head -1; }

ignored=" ${VULN_IGNORE:-} "
failed=0
for id in $called; do
	if [[ $ignored == *" $id "* ]]; then
		echo "ignored $id: $(summary "$id")"
	else
		echo "$id: $(summary "$id") (https://pkg.go.dev/vuln/$id)"
		failed=1
	fi
done

for id in ${VULN_IGNORE:-}; do
	if ! grep -qx "$id" <<<"$called"; then
		echo "$id is ignored but no longer found: take it out of VULN_IGNORE"
		failed=1
	fi
done

if ((failed)); then
	exit 1
fi
echo "No vulnerabilities the code calls, beyond those ignored."
