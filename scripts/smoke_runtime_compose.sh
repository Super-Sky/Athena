#!/usr/bin/env bash
# smoke_runtime_compose.sh validates the local Athena Compose profile and optional live health endpoints.
# smoke_runtime_compose.sh 校验本地 Athena Compose profile 与可选的实时健康端点。
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${root_dir}/deploy/docker-compose.runtime.yml"
env_file="${ATHENA_RUNTIME_ENV_FILE:-${root_dir}/deploy/athena.runtime.env}"
api_url="${ATHENA_RUNTIME_API_URL:-http://127.0.0.1:8080}"

if [[ ! -f "${env_file}" ]]; then
  echo "missing runtime env file: ${env_file}" >&2
  echo "copy deploy/athena.runtime.env.example first or set ATHENA_RUNTIME_ENV_FILE" >&2
  exit 2
fi

docker compose --env-file "${env_file}" -f "${compose_file}" config >/dev/null
echo "compose config: ok"

if [[ "${ATHENA_RUNTIME_LIVE_SMOKE:-0}" != "1" ]]; then
  echo "live smoke skipped; set ATHENA_RUNTIME_LIVE_SMOKE=1 after docker compose up"
  exit 0
fi

curl --fail --silent --show-error "${api_url}/healthz" >/dev/null
echo "api healthz: ok"
