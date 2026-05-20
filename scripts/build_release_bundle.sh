#!/usr/bin/env bash
set -euo pipefail

export LC_ALL=C
export LANG=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/output/release}"
GOOS_VALUE="${GOOS_VALUE:-linux}"
GOARCH_VALUE="${GOARCH_VALUE:-amd64}"

VERSION="${VERSION:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
BRANCH="${BRANCH:-$(git -C "${ROOT_DIR}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)}"
COMMIT="${COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo local)}"
BUILD_TIME="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

BUNDLE_NAME="athena-${VERSION}-${GOOS_VALUE}-${GOARCH_VALUE}"
BUNDLE_DIR="${OUTPUT_DIR}/${BUNDLE_NAME}"
ARCHIVE_PATH="${OUTPUT_DIR}/${BUNDLE_NAME}.tar.gz"

rm -rf "${BUNDLE_DIR}" "${ARCHIVE_PATH}"
mkdir -p "${BUNDLE_DIR}/deploy" "${BUNDLE_DIR}/config"

(
  cd "${ROOT_DIR}"
  CGO_ENABLED=0 GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" go build \
    -ldflags="-s -w \
    -X gitee.com/super_sky/mkh_utils.Version=${VERSION} \
    -X gitee.com/super_sky/mkh_utils.Branch=${BRANCH} \
    -X gitee.com/super_sky/mkh_utils.Commit=${COMMIT} \
    -X gitee.com/super_sky/mkh_utils.BuildTime=${BUILD_TIME}" \
    -o "${BUNDLE_DIR}/athena" .
)

cp -R "${ROOT_DIR}/config/." "${BUNDLE_DIR}/config/"
cp -R "${ROOT_DIR}/deploy/." "${BUNDLE_DIR}/deploy/"
cp "${ROOT_DIR}/README.md" "${BUNDLE_DIR}/README.md"
cp "${ROOT_DIR}/docs/云端部署与交付.md" "${BUNDLE_DIR}/云端部署与交付.md"

tar -C "${OUTPUT_DIR}" -czf "${ARCHIVE_PATH}" "${BUNDLE_NAME}"

echo "Bundle directory: ${BUNDLE_DIR}"
echo "Archive path: ${ARCHIVE_PATH}"
