# Development

## Prerequisites and pinned tools

Install mise and Docker. The repository pins Go 1.27.1, Node.js 24.21.0, and pnpm 10.34.5 in `mise.toml`; `web/package.json` also pins pnpm. Docker is inspected as a target deployment tool but is not used in Phase 0.

```sh
mise install
make install
```

pnpm is the only JavaScript package manager. Commit exactly `web/pnpm-lock.yaml`; do not generate npm or Yarn lockfiles.

## Commands

| Command | Purpose |
|---|---|
| `make format` | Format Go source. |
| `make check` | Verify Go formatting, run `go vet`, and strict TypeScript checking. |
| `make test` | Run Go tests. |
| `make build` | Build the Go executable and production frontend bundle. |
| `make dev-backend` | Run the API on `127.0.0.1:8080`. Override with `-addr` via direct `go run` when needed. |
| `make dev-frontend` | Run Vite's development server. |

Generated `upaste`, `web/dist`, dependencies, runtime databases, uploaded data, and local secrets are ignored. Do not commit them. Before committing run `make format`, `make check`, `make test`, and `make build`, then inspect `git diff`, staged files, and status for secrets or artifacts.

## Endpoint

```sh
curl -i http://127.0.0.1:8080/healthz
```

`GET /healthz` returns HTTP 200 and `application/json; charset=utf-8`. No other application endpoint exists in Phase 0.

## CI

CI repeats formatting, vet, Go tests/build, a frozen pnpm install, TypeScript check, and frontend build. Action references are immutable SHAs annotated with their upstream major tag in the workflow.
