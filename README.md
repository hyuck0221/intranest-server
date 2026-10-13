<p align="center"><img src="docs/assets/intranest-mark.png" width="112" alt="IntraNest logo"></p>

# IntraNest Server

Keep your team's links, files, and conversations on a computer you control. The server runs inside your intranet and stores shared content on its own data volume.

**Current version:** `0.2.0`

**Available languages:** [한국어](README.ko.md)

## Install on your server computer

Install Docker Desktop (Windows/macOS) or Docker Engine (Linux), start it, then run:

```sh
curl -fsSL https://github.com/hyuck0221/intranest-server/releases/latest/download/install.sh | sh
```

This downloads the official container and starts it without cloning or forking the repository. On Windows, run the command in WSL or Git Bash with Docker Desktop running. The server listens on port `43187` across the computer's network interfaces.

In Admin or the Chrome extension, enter only the server computer's IP address or host name, such as `192.168.1.40`. The apps use port `43187` automatically. Allow TCP port `43187` through the host firewall for the network that should use IntraNest.

Clients try HTTPS first and retry over HTTP if the HTTPS connection fails. Configure a trusted certificate on the server when encrypted connections are required.

For a browser page hosted over HTTPS, configure HTTPS on IntraNest too so chat can use a secure WebSocket. Put the certificate and key in a host directory, then point the installer at that directory and their file names inside `/certs`:

```sh
curl -fsSL https://github.com/hyuck0221/intranest-server/releases/latest/download/install.sh | INTRANEST_CERT_DIR="$PWD/certs" INTRANEST_TLS_CERT=/certs/fullchain.pem INTRANEST_TLS_KEY=/certs/privkey.pem sh
```

Use a certificate trusted by client devices and valid for the server address. The installer copies those files into a Docker volume with read-only access for the server.

## What the team can share

- Links to everyday team sites.
- Files stored on the server computer.
- Live chat and its 30-day default history retention.

The host can change chat retention from Admin → Settings. Uploads default to 100 MiB per file and 10 GiB total.

## Network access

IntraNest has no sign-in or access key. Anyone who can reach the server can view, add, and delete links, files, and chat messages. The server accepts browser requests from any origin and listens on all network interfaces by default. Keep it on a trusted network, restrict the port with the computer's firewall, and do not expose it to the public internet.

Chat display names are entered by each sender and are not verified accounts. Standard HTTP does not encrypt traffic; configure HTTPS on a trusted network endpoint when encryption is required.

## Manage the server

- Check it: `docker ps --filter name=intranest-server`
- View logs: `docker logs -f intranest-server`
- Stop it: `docker stop intranest-server`
- Start it again: `docker start intranest-server`

Installer data is kept in the Docker volume `intranest-data`, including the database and uploaded files. Removing the container does not remove this volume.

## Configuration

The installer accepts these optional environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `INTRANEST_VERSION` | `latest` | Container tag to install |
| `INTRANEST_BIND_ADDRESS` | `0.0.0.0` | Host interface to publish |
| `INTRANEST_DATA_VOLUME` | `intranest-data` | Docker volume for the database and files |
| `INTRANEST_CERT_DIR` | — | Host directory containing the TLS files when HTTPS is enabled |
| `INTRANEST_CERT_VOLUME` | `<container>-certs` | Docker volume containing the server's read-only TLS files |
| `INTRANEST_TLS_CERT` | — | Certificate path inside the container, for example `/certs/fullchain.pem` |
| `INTRANEST_TLS_KEY` | — | Private key path inside the container, for example `/certs/privkey.pem` |
| `INTRANEST_MAX_FILE_BYTES` | `104857600` | Per-file upload limit |
| `INTRANEST_MAX_TOTAL_BYTES` | `10737418240` | Total shared-file limit |

The standalone release packages for Linux, macOS, and Windows are also available on [GitHub Releases](https://github.com/hyuck0221/intranest-server/releases). See the [API v2 guide](docs/api-v2.md) for developers.

## Release versions

The root `version` file controls this project's version. A push to `master` publishes a GitHub Release, installer script, native packages, and multi-architecture container only when that version is newer than the latest published release. The server, web, and extension versions are managed separately.
