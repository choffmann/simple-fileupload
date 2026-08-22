# simple-fileupload

A small file server for sharing files over a link or a QR code. Every user gets
their own area, signs in through an OIDC provider and uploads into that area.
Browsing and downloading needs no login at all, so a link or a scanned QR code is
enough for anyone to get at the files.

The whole thing is a single Go binary with embedded templates, no database and no
client side JavaScript. Files live on disk under `UPLOAD_DIR/<username>`, which is
also what the public URLs mirror.

## Features

Signing in creates the area for the `preferred_username` claim and drops the user
into it. From there they can upload several files in one go, create folders and
generate a QR code for any file or folder. Visitors without a session see the same
directory listing, follow the same links and get the same QR pages, they just have
no upload form.

Uploaded files are served from the application's own origin, so responses carry
`X-Content-Type-Options: nosniff` and a restrictive `Content-Security-Policy`. That
keeps an uploaded HTML or SVG file from running as first party script against a
visitor's session. Uploads, folder creation and logout additionally require a
matching `Origin` header.

## Routes

| Route | Purpose |
| --- | --- |
| `GET /` | list of all areas, redirects to your own area when signed in |
| `GET /{username}/{path...}` | directory listing or file download |
| `GET /qr/{username}/{path...}` | QR page, `?format=png` for the image itself |
| `POST /upload` | multipart upload into your own area, requires a session |
| `POST /mkdir` | create a folder in your own area, requires a session |
| `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout` | OIDC login flow |

## Configuration

Everything is read from the environment at startup. A missing or malformed value
in one of the required variables aborts the start instead of being guessed.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `BASE_URL` | yes | | Public base URL, for example `https://files.example.com`. QR codes are absolute, so this cannot be derived from request headers. An `https://` value also switches the cookies to `Secure`. |
| `OIDC_ISSUER` | yes | | Issuer URL of the identity provider, used for discovery at startup. |
| `OIDC_CLIENT_ID` | yes | | Client ID of the confidential client. |
| `OIDC_CLIENT_SECRET` | yes | | Client secret. |
| `UPLOAD_DIR` | no | `./data` | Directory holding the user areas. |
| `SESSION_SECRET` | no | random | Signing key for the session cookie, at least 16 characters. Without it a random key is generated and every restart signs all users out. |
| `LOG_LEVEL` | no | `info` | One of `debug`, `info`, `warn`, `error`. |
| `LOG_FORMAT` | no | `text` | `text` or `json`. |

The redirect URI to register with the provider is `BASE_URL` plus
`/auth/callback`. The server always listens on port `8080`. Uploads are limited to
200 MiB per request, of which 32 MiB are buffered in memory before the rest spills
into temporary files. Sessions are valid for 12 hours.

## Running it

Images are published to the GitHub Container Registry. The `edge` tag follows
`main`, releases are tagged with their semantic version and with `latest`.

```bash
docker run --rm -p 8080:8080 \
  -v ./data:/data \
  -e BASE_URL=https://files.example.com \
  -e UPLOAD_DIR=/data \
  -e OIDC_ISSUER=https://sso.example.com/realms/example \
  -e OIDC_CLIENT_ID=simple-fileupload \
  -e OIDC_CLIENT_SECRET=... \
  -e SESSION_SECRET=... \
  ghcr.io/choffmann/simple-fileupload:edge
```

## Development

The repository ships a Nix flake with the Go toolchain, `golangci-lint` and the
rest of the tooling. With direnv it is `direnv allow`, otherwise `nix develop`.

For a login you need a provider. The `compose.yaml` brings up a Keycloak with a
preconfigured realm for exactly that, on `http://localhost:8081`, with the users
`alice` and `bob` whose passwords match their usernames.

```bash
docker compose up -d

export BASE_URL=http://localhost:8080
export OIDC_ISSUER=http://localhost:8081/realms/simple-fileupload
export OIDC_CLIENT_ID=simple-fileupload
export OIDC_CLIENT_SECRET=dev-secret-not-for-production

go run .
```

The realm's redirect URI is pinned to `http://localhost:8080/auth/callback`, so the
`BASE_URL` above is the one that works out of the box.

```bash
go test -race ./...
golangci-lint run
```

## License

MIT, see [LICENSE](LICENSE).
