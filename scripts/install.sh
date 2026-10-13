#!/bin/sh
set -eu

image="${INTRANEST_IMAGE:-ghcr.io/hyuck0221/intranest-server}"
version="${INTRANEST_VERSION:-latest}"
container="${INTRANEST_CONTAINER_NAME:-intranest-server}"
volume="${INTRANEST_DATA_VOLUME:-intranest-data}"
cert_volume="${INTRANEST_CERT_VOLUME:-${container}-certs}"
port=43187
bind_address="${INTRANEST_BIND_ADDRESS:-0.0.0.0}"
cert_dir="${INTRANEST_CERT_DIR:-}"
tls_cert="${INTRANEST_TLS_CERT:-}"
tls_key="${INTRANEST_TLS_KEY:-}"
tls_stage=""

case "$version" in
	''|*[!A-Za-z0-9_.-]*)
		echo "INTRANEST_VERSION must be a Docker tag such as latest or 0.2.0." >&2
		exit 1
		;;
esac
if [ -n "$tls_cert" ] || [ -n "$tls_key" ]; then
	if [ -z "$tls_cert" ] || [ -z "$tls_key" ] || [ -z "$cert_dir" ]; then
		echo "Set INTRANEST_CERT_DIR, INTRANEST_TLS_CERT, and INTRANEST_TLS_KEY together to enable HTTPS." >&2
		exit 1
	fi
	if [ ! -d "$cert_dir" ]; then
		echo "INTRANEST_CERT_DIR must point to a directory containing your TLS certificate and key." >&2
		exit 1
	fi
	cert_dir=$(cd -P "$cert_dir" && pwd)
	case "$tls_cert" in
		/certs/*) tls_cert_name=${tls_cert#/certs/} ;;
		*) echo "INTRANEST_TLS_CERT must be a file directly under /certs." >&2; exit 1 ;;
	esac
	case "$tls_key" in
		/certs/*) tls_key_name=${tls_key#/certs/} ;;
		*) echo "INTRANEST_TLS_KEY must be a file directly under /certs." >&2; exit 1 ;;
	esac
	for file_name in "$tls_cert_name" "$tls_key_name"; do
		case "$file_name" in
			''|.|..|*/*|*[!A-Za-z0-9_.-]*)
				echo "TLS filenames may contain only letters, numbers, dots, underscores, and hyphens." >&2
				exit 1
				;;
		esac
		if [ ! -f "$cert_dir/$file_name" ]; then
			echo "TLS file not found: $cert_dir/$file_name" >&2
			exit 1
		fi
	done
	if [ "$tls_cert_name" = "$tls_key_name" ]; then
		echo "Use separate files for INTRANEST_TLS_CERT and INTRANEST_TLS_KEY." >&2
		exit 1
	fi
fi

if ! command -v docker >/dev/null 2>&1; then
	echo "Install Docker Desktop or Docker Engine, then run this installer again." >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "Docker is installed but not running. Start Docker and run this installer again." >&2
	exit 1
fi

echo "Downloading ${image}:${version}..."
docker pull "${image}:${version}"
docker volume create "$volume" >/dev/null
if [ -n "$tls_cert" ]; then
	tls_stage=$(mktemp -d "${TMPDIR:-/tmp}/intranest-tls.XXXXXX")
	trap 'rm -rf "$tls_stage"' EXIT
	cp -L "$cert_dir/$tls_cert_name" "$tls_stage/$tls_cert_name"
	cp -L "$cert_dir/$tls_key_name" "$tls_stage/$tls_key_name"
	docker volume create "$cert_volume" >/dev/null
	docker run --rm \
		--volume "${tls_stage}:/source:ro" \
		--volume "${cert_volume}:/certs" \
		busybox:1.37 sh -ec 'cp "/source/$1" "/certs/$1"; cp "/source/$2" "/certs/$2"; chown 65532:65532 "/certs/$1" "/certs/$2" /certs; chmod 0400 "/certs/$1" "/certs/$2"; chmod 0500 /certs' sh "$tls_cert_name" "$tls_key_name"
fi

if docker container inspect "$container" >/dev/null 2>&1; then
	docker rm -f "$container" >/dev/null
fi

set -- \
	--name "$container" \
	--restart unless-stopped \
	--publish "${bind_address}:${port}:${port}" \
	--volume "${volume}:/data" \
	--env INTRANEST_LISTEN_ADDR=0.0.0.0:43187 \
	--env INTRANEST_DATA_DIR=/data \
	--env "INTRANEST_MAX_FILE_BYTES=${INTRANEST_MAX_FILE_BYTES:-104857600}" \
	--env "INTRANEST_MAX_TOTAL_BYTES=${INTRANEST_MAX_TOTAL_BYTES:-10737418240}"
if [ -n "$tls_cert" ]; then
	set -- "$@" \
		--volume "${cert_volume}:/certs:ro" \
		--env "INTRANEST_TLS_CERT=${tls_cert}" \
		--env "INTRANEST_TLS_KEY=${tls_key}"
fi
docker run -d "$@" "${image}:${version}" >/dev/null

echo "IntraNest Server ${version} is running on port ${port}."
echo "Enter this computer's IP address or host name in Admin or the Chrome extension."
echo "Clients use HTTPS first, then HTTP if needed, on port ${port}."
echo "Data remains in Docker volume: ${volume}"
echo "Check status: docker ps --filter name=${container}"
echo "Stop server: docker stop ${container}"
echo "Use a trusted intranet and do not expose this unauthenticated server to the public internet."
