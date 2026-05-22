#!/usr/bin/env bash
set -euo pipefail

image="${1:-${MINTD_CONFIDENTIAL_IMG:-cashu-mintd-gke-confidential:dev}}"

run_expect_failure() {
	local label="$1"
	local expected="$2"
	shift 2

	local output status
	set +e
	output="$("$@" 2>&1)"
	status=$?
	set -e

	if [ "$status" -eq 0 ]; then
		printf '%s\n' "$output"
		printf 'expected %s to fail, but it exited 0\n' "$label" >&2
		exit 1
	fi
	if [[ "$output" != *"$expected"* ]]; then
		printf '%s\n' "$output"
		printf 'expected %s output to contain: %s\n' "$label" "$expected" >&2
		exit 1
	fi

	printf 'ok: %s failed closed (%s)\n' "$label" "$expected"
}

run_expect_failure \
	missing-config \
	"is required" \
	docker run --rm "$image"

run_expect_failure \
	missing-vtpm \
	"vTPM device /dev/tpmrm0 unavailable" \
	docker run --rm \
		-e CASHU_ATTESTATION_PROJECT_ID=p \
		-e CASHU_ATTESTATION_LOCATION=us-central1 \
		-e CASHU_ATTESTATION_WORKLOAD_IDENTITY_PROVIDER=//iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/pool/providers/provider \
		-e CASHU_ATTESTATION_EXPECTED_SERVICE_ACCOUNT=svc@example.iam.gserviceaccount.com \
		-e CASHU_MNEMONIC_SECRET_VERSION=projects/p/secrets/s/versions/latest \
		"$image"

printf 'ok: confidential wrapper smoke checks passed for %s\n' "$image"
