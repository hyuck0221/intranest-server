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
ENV
  chmod 600 .env
fi

docker compose pull
docker compose up -d

echo "IntraNest server is starting on port 43187."
echo "Enter the server computer's IP address in Admin or the extension."
echo "Anyone who can reach this address can manage all shared content. Keep the port on a trusted network."
echo "Check status with: docker compose ps"
