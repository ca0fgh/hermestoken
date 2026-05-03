FRONTEND_CLASSIC_DIR = ./web/classic
FRONTEND_DEFAULT_DIR = ./web/default
FRONTEND_DIR = $(FRONTEND_CLASSIC_DIR)
BACKEND_DIR = .

.PHONY: all build-frontend build-frontend-default build-frontend-classic build-all-frontends start-backend dev dev-api dev-web dev-web-default dev-web-classic

all: build-all-frontends start-backend

build-frontend:
	@echo "Building classic frontend..."
	@cd $(FRONTEND_DIR) && bun install && VITE_REACT_APP_VERSION=$$(cat ../../VERSION) bun run build

build-frontend-default:
	@echo "Building compatibility frontend..."
	@cd $(FRONTEND_DEFAULT_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$$(cat ../../VERSION) bun run build

build-frontend-classic: build-frontend

build-all-frontends: build-frontend build-frontend-default

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

dev-api:
	@echo "Starting backend services (docker)..."
	@docker compose -f docker-compose.dev.yml up -d

dev-web:
	@echo "Starting classic frontend dev server..."
	@cd $(FRONTEND_DIR) && bun install && bun run dev

dev-web-default:
	@echo "Starting compatibility frontend dev server..."
	@cd $(FRONTEND_DEFAULT_DIR) && bun install && bun run dev

dev-web-classic: dev-web

dev: dev-api dev-web
