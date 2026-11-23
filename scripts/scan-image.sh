#!/bin/sh
# Scan the built image for known vulnerabilities with trivy.
#
#   ./scripts/scan-image.sh [image]        default akmesh:latest
#
# The image is exported to a tar first: trivy then reads exactly what podman
# stored, without needing a socket or a registry. Suppressions live in
# .trivyignore at the repository root, next to the reason for each one.
set -e
IMG=${1:-akmesh:latest}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${TMPDIR:-/tmp}/akmesh-scan-$$.tar
trap 'rm -f "$OUT"' EXIT

command -v trivy >/dev/null 2>&1 || { echo "scan-image: trivy is not installed" >&2; exit 2; }
command -v podman >/dev/null 2>&1 || { echo "scan-image: podman is not installed" >&2; exit 2; }

podman save -o "$OUT" "localhost/$IMG" 2>/dev/null || podman save -o "$OUT" "$IMG"
echo "scan-image: $IMG with trivy $(trivy --version | awk '/^Version/{print $2}')"
cd "$ROOT"
exec trivy image --input "$OUT" --scanners vuln,secret --quiet --ignorefile "$ROOT/.trivyignore"
