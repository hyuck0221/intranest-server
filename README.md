# IntraNest Server

Keep your team's shared links, files, and conversations on a computer inside your intranet. IntraNest Server runs on your own machine; it does not upload this content to a public hosting service.

**Current version:** `0.1.0`  
**Available languages:** [한국어](README.ko.md)

## What it does

- Keeps shared links, chat history, and uploaded files on the server computer.
- Lets browser extensions and the IntraNest web admin connect to one intranet address.
- Removes chat messages after 30 days by default. The host can change this in Admin → Settings.
- Provides Docker Compose and standalone downloads for Windows, macOS, and Linux.
- Protects the host disk with a 100 MiB per-file limit and a 10 GiB shared-file limit by default.

## Start with Docker Compose

1. Install Docker Desktop (Windows/macOS) or Docker Engine with Compose v2 (Linux).
2. Download this repository on the computer that will host the server.
3. Run the setup script:

   ```sh
   ./scripts/setup.sh
   ```

   On Windows, run the same steps in WSL, or start the container with Docker Desktop using the compose file.
4. If your public IntraNest website is hosted at a different origin, add that exact origin to `INTRANEST_CORS_ORIGINS` in `.env`, then restart the server.
5. Allow TCP port `8080` through the host computer's firewall for your intranet only.
6. After the first start, share the access key from `data/access.token` with your team through a trusted channel.
7. In IntraNest Admin or the extension, enter `http://<server-computer-address>:8080` and the access key.

The connection check is public so a browser can confirm that the server is present. Links, files, chat, and settings require the access key.

## Install without Docker

Download the latest package for the host computer from [GitHub Releases](https://github.com/hyuck0221/intranest-server/releases), extract it, and start the executable from that folder. The server creates a `data` folder and a random access key on first launch.

- macOS/Linux: `./intranest`
- Windows: `intranest.exe`

The server listens on `0.0.0.0:8080` by default. Read the key in `data/access.token` and allow the port through the computer's intranet firewall.

## Keep the connection private

The access key is a shared team key, not an individual account. Anyone who has it can read, add, and delete shared content. Keep it private and rotate it by stopping the server, replacing `data/access.token` with a new random value of at least 32 characters, then restarting.

Names shown beside chat messages are entered by each sender and are not verified accounts.

The server supports HTTPS when you provide a trusted certificate and key with `INTRANEST_TLS_CERT` and `INTRANEST_TLS_KEY`. Without HTTPS, API traffic and the access key are not encrypted on the local network. Use a trusted internal certificate when the network is not fully trusted. Set `INTRANEST_CORS_ORIGINS` to the exact origin of your IntraNest web page; do not use `*`.

Chrome may ask the web page for permission to connect to local network devices. Grant it for the IntraNest site to use the address you entered. The extension asks Chrome for access only to the server origin you configure.

## API

The server exposes a versioned JSON API under `/api/v1`. See [API v1](docs/api-v1.md) for the connection check, access key, links, chat, files, and settings endpoints.

## Data and configuration

By default, `./data` contains the database, shared files, and access key. Back up this folder while the server is stopped. You can change the main settings with environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `INTRANEST_LISTEN_ADDR` | `0.0.0.0:8080` | Interface and port to listen on |
| `INTRANEST_DATA_DIR` | `./data` | Database, uploaded files, and access key folder |
| `INTRANEST_ACCESS_TOKEN` | generated on first start | Override the shared access key |
| `INTRANEST_CORS_ORIGINS` | no web origins | Comma-separated exact web origins allowed to call the API |
| `INTRANEST_MAX_FILE_BYTES` | `104857600` | Per-file limit (100 MiB; accepted range 1 MiB–2 GiB) |
| `INTRANEST_MAX_TOTAL_BYTES` | `10737418240` | Total shared file storage limit (10 GiB; minimum 1 MiB) |
| `INTRANEST_TLS_CERT` / `INTRANEST_TLS_KEY` | unset | Certificate and key paths; set both to enable HTTPS |

The chat retention setting defaults to 30 days and can be changed from 1 to 3,650 days in Admin → Settings. File storage is limited to 10 GiB by default; set `INTRANEST_MAX_TOTAL_BYTES` to change it.

## Release versions

The root `version` file controls this project's version. A push to `master` publishes a GitHub Release and multi-architecture container only when that version is newer than the latest published release. The server has its own version, separate from the web and extension projects.
