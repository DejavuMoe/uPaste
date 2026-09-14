.PHONY: format check test build dev-backend dev-frontend install

format:
	mise exec -- gofmt -w cmd

check:
	mise exec -- gofmt -d cmd
	mise exec -- go vet ./...
	mise exec -- pnpm --dir web check

test:
	mise exec -- go test ./...

build:
	mise exec -- go build ./cmd/upaste
	mise exec -- pnpm --dir web build

install:
	mise exec -- pnpm --dir web install --frozen-lockfile

dev-backend:
	mise exec -- go run ./cmd/upaste

dev-frontend:
	mise exec -- pnpm --dir web dev
