.PHONY: format check test build e2e prod-e2e prove-embedded dev-backend dev-frontend install clean

format:
	mise exec -- ./scripts/gofmt write

check:
	mise exec -- ./scripts/gofmt check
	mise exec -- go vet ./...
	mise exec -- pnpm --dir web check

test:
	mise exec -- go test ./...
	mise exec -- pnpm --dir web test

# Complete production build: TypeScript check + Vite build, clean embed staging,
# then Go compilation with -tags production. Produces ./upaste.
build:
	mise exec -- ./scripts/build-production-binary.sh

e2e:
	mise exec -- pnpm --dir web e2e

prod-e2e:
	mise exec -- pnpm --dir web e2e:production

prove-embedded: build
	mise exec -- ./scripts/prove-embedded-binary.sh ./upaste

install:
	mise exec -- pnpm --dir web install --frozen-lockfile

dev-backend:
	mise exec -- go run ./cmd/upaste

dev-frontend:
	mise exec -- pnpm --dir web dev

clean:
	rm -rf internal/webapp/dist web/dist upaste web/playwright-report web/test-results
