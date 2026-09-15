.PHONY: format check test build e2e prod-e2e prove-embedded dist verify-dist deploy-check qualify-release fuzz benchmark load-smoke dev-backend dev-frontend install clean

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

# Deterministic Linux amd64/arm64 release archives under ./release.
# Usage: make dist VERSION=v0.1.0
dist:
	@test -n "$(VERSION)" || (echo "usage: make dist VERSION=v0.1.0" >&2; exit 2)
	mise exec -- ./scripts/package-release.sh "$(VERSION)"

# Verify checksums, metadata, AArch64 identity, amd64 runtime, restart
# persistence, and deterministic rebuild. Usage: make verify-dist VERSION=v0.1.0
verify-dist:
	@test -n "$(VERSION)" || (echo "usage: make verify-dist VERSION=v0.1.0" >&2; exit 2)
	mise exec -- ./scripts/verify-release.sh "$(VERSION)"

# Validate systemd unit and two-origin reverse-proxy examples.
deploy-check:
	mise exec -- ./scripts/check-deployment.sh

# Complete deterministic pre-release gate: Go+race suites, deployment checks,
# deterministic packages, packaged runtime, backup/restore, and shutdown-load.
# Usage: make qualify-release VERSION=v0.0.0-test
qualify-release:
	@test -n "$(VERSION)" || (echo "usage: make qualify-release VERSION=v0.0.0-test" >&2; exit 2)
	mise exec -- ./scripts/qualify-release.sh "$(VERSION)"

# Bounded local/pre-release fuzz campaign (not every CI run).
fuzz:
	mise exec -- ./scripts/fuzz.sh

# Repeatable local performance baseline (informational, not CI-gated).
benchmark:
	mise exec -- ./scripts/benchmark-local.sh

# Bounded concurrent-read/create/upload/download load smoke (local/pre-release).
load-smoke:
	mise exec -- ./scripts/load-smoke.sh

install:
	mise exec -- pnpm --dir web install --frozen-lockfile

dev-backend:
	mise exec -- go run ./cmd/upaste

dev-frontend:
	mise exec -- pnpm --dir web dev

clean:
	rm -rf internal/webapp/dist web/dist upaste release web/playwright-report web/test-results
