.PHONY: build backend-build backend-vuln cli-build frontend-build clean dev ensure-web-deps lint backend-lint cli-lint frontend-lint test backend-test cli-test frontend-test e2e frontend-e2e

BACKEND_ADDR ?= :8080
FRONTEND_PORT ?= 5173
PNPM ?= $(shell if command -v pnpm >/dev/null 2>&1; then command -v pnpm; else printf 'corepack pnpm'; fi)

build: frontend-build backend-build cli-build

backend-build:
	cd backend && go build ./...

cli-build:
	cd cli && go build ./...

frontend-build: ensure-web-deps
	pnpm --dir web run build

clean:
	rm -rf web/dist backend/accounting-server cli/accounting

dev: ensure-web-deps
	@set -e; \
	backend_addr="$(BACKEND_ADDR)"; \
	case "$$backend_addr" in \
		:*) api_base_url="http://localhost$$backend_addr" ;; \
		*) api_base_url="http://$$backend_addr" ;; \
	esac; \
	VITE_API_BASE_URL="$$api_base_url" $(PNPM) --dir web run dev --host 0.0.0.0 --port "$(FRONTEND_PORT)"

ensure-web-deps:
	@if [ ! -d web/node_modules ]; then $(PNPM) --dir web install; fi

lint: backend-lint cli-lint frontend-lint

backend-lint:
	cd backend && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
	cd backend && go mod tidy
	cd backend && go vet ./...
	cd backend && go tool golangci-lint run ./...

backend-vuln:
	cd backend && go tool govulncheck ./...

cli-lint:
	cd cli && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
	cd cli && go mod tidy
	cd cli && go vet ./...

frontend-lint: ensure-web-deps
	$(PNPM) --dir web run format:check
	$(PNPM) --dir web run lint
	$(PNPM) --dir web run lint:css
	$(PNPM) --dir web run lint:dead
	$(PNPM) --dir web run check:i18n
	$(PNPM) --dir web run check:api

test: backend-test cli-test frontend-test

backend-test:
	cd backend && go test -race -cover ./...

cli-test:
	cd cli && go test -race -cover ./...

frontend-test: ensure-web-deps
	$(PNPM) --dir web run test

e2e: frontend-e2e

frontend-e2e: ensure-web-deps
	$(PNPM) --dir web run test:e2e
