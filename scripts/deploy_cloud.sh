#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${ROOT_DIR}/output/deploy"
PACKAGE_DIR="${WORK_DIR}/package"

DEPLOY_USER="${DEPLOY_USER:?DEPLOY_USER is required}"
DEPLOY_HOST="${DEPLOY_HOST:?DEPLOY_HOST is required}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
DEPLOY_DIR="${DEPLOY_DIR:-/opt/athena}"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/deploy/athena.env}"
INCLUDE_WEB="${INCLUDE_WEB:-false}"
ATHENA_IMAGE="${ATHENA_IMAGE:-athena:local}"
ATHENA_WEB_IMAGE="${ATHENA_WEB_IMAGE:-athena-web:local}"

IMAGE_ARCHIVE="${WORK_DIR}/athena-image.tar.gz"
WEB_IMAGE_ARCHIVE="${WORK_DIR}/athena-web-image.tar.gz"
PACKAGE_ARCHIVE="${WORK_DIR}/athena-deploy-package.tar.gz"
REMOTE_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Missing env file: ${ENV_FILE}" >&2
  echo "Create it from deploy/athena.env.example first." >&2
  exit 1
fi

rm -rf "${WORK_DIR}"
mkdir -p "${PACKAGE_DIR}"

echo "[1/6] Building Athena API image: ${ATHENA_IMAGE}"
docker build -t "${ATHENA_IMAGE}" "${ROOT_DIR}"

echo "[2/6] Saving Athena API image"
docker save "${ATHENA_IMAGE}" | gzip > "${IMAGE_ARCHIVE}"

if [[ "${INCLUDE_WEB}" == "true" ]]; then
  echo "[3/6] Building Athena Web image: ${ATHENA_WEB_IMAGE}"
  docker build -t "${ATHENA_WEB_IMAGE}" -f "${ROOT_DIR}/web/Dockerfile" "${ROOT_DIR}/web"
  echo "[4/6] Saving Athena Web image"
  docker save "${ATHENA_WEB_IMAGE}" | gzip > "${WEB_IMAGE_ARCHIVE}"
else
  echo "[3/6] Skipping Athena Web image build"
fi

cp "${ROOT_DIR}/deploy/docker-compose.cloud.yml" "${PACKAGE_DIR}/docker-compose.cloud.yml"
cp "${ENV_FILE}" "${PACKAGE_DIR}/athena.env"
tar -C "${PACKAGE_DIR}" -czf "${PACKAGE_ARCHIVE}" .

echo "[5/6] Checking remote Docker Compose"
ssh -p "${DEPLOY_PORT}" "${REMOTE_TARGET}" "docker compose version >/dev/null"

echo "[6/6] Uploading and deploying to ${REMOTE_TARGET}:${DEPLOY_DIR}"
scp -P "${DEPLOY_PORT}" "${IMAGE_ARCHIVE}" "${PACKAGE_ARCHIVE}" "${REMOTE_TARGET}:/tmp/"

REMOTE_SCRIPT=$(cat <<EOF
set -euo pipefail
mkdir -p "${DEPLOY_DIR}"
tar -xzf /tmp/athena-deploy-package.tar.gz -C "${DEPLOY_DIR}"
gunzip -c /tmp/athena-image.tar.gz | docker load
if [[ "${INCLUDE_WEB}" == "true" ]]; then
  gunzip -c /tmp/athena-web-image.tar.gz | docker load
  COMPOSE_PROFILES=control-plane ATHENA_IMAGE="${ATHENA_IMAGE}" ATHENA_WEB_IMAGE="${ATHENA_WEB_IMAGE}" docker compose --env-file "${DEPLOY_DIR}/athena.env" -f "${DEPLOY_DIR}/docker-compose.cloud.yml" up -d
else
  ATHENA_IMAGE="${ATHENA_IMAGE}" ATHENA_WEB_IMAGE="${ATHENA_WEB_IMAGE}" docker compose --env-file "${DEPLOY_DIR}/athena.env" -f "${DEPLOY_DIR}/docker-compose.cloud.yml" up -d
fi
EOF
)

if [[ "${INCLUDE_WEB}" == "true" ]]; then
  scp -P "${DEPLOY_PORT}" "${WEB_IMAGE_ARCHIVE}" "${REMOTE_TARGET}:/tmp/"
fi

ssh -p "${DEPLOY_PORT}" "${REMOTE_TARGET}" "${REMOTE_SCRIPT}"

echo "Deployment completed."
echo "Remote path: ${DEPLOY_DIR}"
