# Local development

Requires Go 1.26+, Bun 1.3.9, and Python 3. Run commands from the repository root.

Sign-in requires the Cognito configuration from [deployment setup](deployment.md).

## 1. Install dependencies

```sh
bun install --cwd src/web --frozen-lockfile
```

## 2. Start the API

Set the authentication settings and authorized Google email:

```sh
export APP_ORIGIN="http://localhost:5173"
export OIDC_ISSUER="<OIDC_ISSUER from Lambda>"
export OIDC_CLIENT_ID="<OIDC_CLIENT_ID from Lambda>"
export AUTH_DOMAIN="<AUTH_DOMAIN from Lambda>"
export OWNER_EMAIL="<your authorized Google email>"
export COOKIE_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
```

Build the frontend once so the API can load its assets, then start the API:

```sh
bun run --cwd src/web --bun build
go run ./src/api/cmd/server
```

The API listens on `http://localhost:8080`.

## 3. Start the frontend

In another terminal:

```sh
bun run --cwd src/web --bun dev
```

Open **http://localhost:5173** and sign in with Google.

Frontend edits reload automatically. Restart the API after Go changes.
