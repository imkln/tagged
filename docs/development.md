# Local development

Install Go 1.26+, Bun 1.3.9, Python 3, and Make. Commands below start from the repository root.

Tests can run immediately. To sign in to the running app, first complete [first-time setup and deployment](deployment.md).

## 1. Install dependencies and run tests

```sh
cd src/web
bun install --frozen-lockfile
cd ../..
make test
```

## 2. Get the deployed authentication settings

1. Open the AWS console and select **US East (N. Virginia) / us-east-1**.
2. Open **Lambda → Functions → tagged-api → Configuration → Environment variables**.
3. Copy `OIDC_ISSUER`, `OIDC_CLIENT_ID`, and `AUTH_DOMAIN` for the next section. `OIDC_CLIENT_ID` is the Cognito app client ID, not the Google client ID.

## 3. Start the API

In your first terminal, replace the placeholders below with those three values and your authorized Google email:

```sh
export APP_ORIGIN="http://localhost:5173"
export OIDC_ISSUER="<OIDC_ISSUER from Lambda>"
export OIDC_CLIENT_ID="<OIDC_CLIENT_ID from Lambda>"
export AUTH_DOMAIN="<AUTH_DOMAIN from Lambda>"
export OWNER_EMAIL="<your authorized Google email>"
export COOKIE_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
```

Build the frontend once, then start the API. The API needs the built files at startup; Vite supplies live updates during development.

```sh
cd src/web
bun run --bun build
cd ../..
go run ./src/api/cmd/server
```

Keep this terminal running. The API listens on `127.0.0.1:8080`.

## 4. Start the frontend

In a second terminal, open the repository root and run:

```sh
cd src/web
bun run --bun dev
```

Open **http://localhost:5173** and click **Continue with Google**. Use `localhost`, not `127.0.0.1`: the registered callback is `http://localhost:5173/auth/callback`.

Frontend edits update in the browser. Restart `go run ./src/api/cmd/server` after changing Go code. Press **Ctrl+C** in each terminal to stop the app.

## Format changes

Run from the repository root:

```sh
gofmt -w src/api
cd src/web
bun run --bun format
bun run --bun prettier --write '../../.github/workflows/*.yml' ../../script/deployment-policy.json
```

For Terraform formatting, open a terminal at the repository root and run:

```sh
cd infra
terraform fmt
```

Terraform is only needed locally for editing infrastructure; application deployment runs through GitHub Actions.
