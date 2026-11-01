# Development

The frontend is React and TypeScript, built with Vite and managed with Bun. The Go API handles authentication and serves the frontend build. Terraform defines the AWS deployment.

```text
src/
  api/                Go HTTP handlers and tests
    cmd/server/       Local server and Lambda entry point
  web/
    components/       Shared interface components and their styles
    hooks/            Session loading
    pages/            Pages and their styles
infra/                Terraform
script/               Setup and release packaging
```

## Prerequisites

- Go 1.26 or later.
- Bun 1.3.9.
- Terraform 1.10 or later, below 2.0.
- Python 3 for packaging the Lambda artifact.
Tests do not require AWS or Google credentials.

## Install and test

From the repository root:

```sh
cd src/web
bun install --frozen-lockfile
cd ../..
make test
```

The backend tests use a local identity provider with signed tokens. Frontend tests mock API responses. 

## Run locally

Real sign-in requires the Cognito and Google configuration described in [deployment](deployment.md). The same Cognito client accepts the production callback and `http://localhost:5173/auth/callback`.

Set these environment variables in the API terminal; do not commit their values:

| Variable         | Value                                                           |
| ---------------- | --------------------------------------------------------------- |
| `APP_ORIGIN`     | `http://localhost:5173`                                         |
| `OIDC_ISSUER`    | Terraform's `oidc_issuer` output                                |
| `OIDC_CLIENT_ID` | Terraform's `cognito_client_id` output                          |
| `AUTH_DOMAIN`    | Terraform's `auth_domain` output                                |
| `OWNER_EMAIL`    | The verified Google email authorized for this library           |
| `OWNER_SUBJECT`  | Optional Cognito subject, if configured in deployment           |
| `COOKIE_KEY`     | A local 32-byte random key encoded as 64 hexadecimal characters |

Generate a local cookie key without writing it to a file:

```sh
export COOKIE_KEY="$(openssl rand -hex 32)"
```

Build the frontend once, then start the API from the repository root:

```sh
cd src/web
bun run --bun build
cd ../..
go run ./src/api/cmd/server
```

In another terminal:

```sh
cd src/web
bun run --bun dev
```

Open `http://localhost:5173`. Vite forwards `/api` and `/auth` to the Go server on `127.0.0.1:8080`. Use `localhost`, matching the registered callback, rather than `127.0.0.1` in the browser. Local HTTP cookies omit `Secure`; HTTPS deployment cookies include it.

## Formatting

```sh
gofmt -w src/api
cd infra
terraform fmt
cd ..
cd src/web
bun run --bun format
bun run --bun prettier --write '../../.github/workflows/*.yml' ../../script/deployment-policy.json
```
