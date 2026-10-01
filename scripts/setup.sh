#!/bin/sh
set -eu

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install Docker Desktop or Docker Engine first." >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required." >&2
  exit 1
fi

mkdir -p data certs
chmod 700 data
if [ ! -f .env ]; then
  cat > .env <<ENV
INTRANEST_UID=$(id -u)
INTRANEST_GID=$(id -g)
INTRANEST_CORS_ORIGINS=http://localhost:4173,http://127.0.0.1:4173
ENV
  chmod 600 .env
fi

docker compose pull
docker compose up -d

echo "IntraNest server is starting on port ${INTRANEST_PORT:-8080}."
echo "After the first start, find the access key in ./data/access.token and share it securely with your team."
echo "Check status with: docker compose ps"
