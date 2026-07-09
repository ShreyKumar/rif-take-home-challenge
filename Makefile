# Convenience targets for the mutant-detector service.
# The backend is a self-contained Go module under backend/.

.PHONY: help build run test stress loadtest

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ## Build the server binary to backend/bin/server
	cd backend && go build -o bin/server ./cmd/server

run: ## Run the server (config via PORT, DB_PATH, FRONTEND_DIR)
	cd backend && go run ./cmd/server

test: ## Run all backend tests under -race with coverage
	cd backend && go test -race -covermode=atomic -coverpkg=./internal/... -coverprofile=coverage.out ./...
	cd backend && go tool cover -func=coverage.out | tail -1

stress: ## Run the concurrency stress test under -race
	cd backend && go test -race -run TestConcurrent ./internal/api/

loadtest: ## Build + start the server and drive load (reads + mixed); see loadtest/RESULTS.md
	bash loadtest/run.sh
