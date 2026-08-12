# MarketConnector demo Makefile
#
# Quick start:
#   make run            # run with the broker set in .env
#   make run-angelone   # force Angel One
#   make run-zerodha    # force Zerodha (opens browser login)
#   make run ARGS="-ws" # pass extra flags (e.g. WebSocket demo)
#   make build test vet tidy

.PHONY: help run run-angelone run-zerodha build test vet tidy

# Extra CLI args forwarded to the binary (e.g. ARGS="-ws").
ARGS ?=

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

run: ## Run the demo CLI using the broker from .env
	go run ./cmd/marketconnector $(ARGS)

run-angelone: ## Run the demo CLI against Angel One
	BROKER=angelone go run ./cmd/marketconnector $(ARGS)

run-zerodha: ## Run the demo CLI against Zerodha (browser login)
	BROKER=zerodha go run ./cmd/marketconnector $(ARGS)

build: ## Compile the whole module
	go build ./...

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod / go.sum
	go mod tidy
