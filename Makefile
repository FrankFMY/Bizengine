.PHONY: build run dev test test-integration test-coverage migrate-up migrate-down lint clean gen-types install-hooks seed dev-full watch-types

BIN=./bin/engine
CMD=./cmd/server
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

-include .env
export

build:
	go build -o $(BIN) $(CMD)

run: build
	$(BIN)

dev:
	air

test:
	go test ./... -race -count=1

test-integration:
	go test ./... -race -count=1 -tags=integration

test-coverage:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

migrate-up:
	migrate -path ./migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DB_URL)" down 1

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir ./migrations -seq $$name

lint:
	go vet ./...

gen-types:
	go run ./cmd/gen-types/

seed:
	go run ./cmd/seed/

dev-full:
	docker compose up -d
	@sleep 3
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) dev

watch-types:
	@echo "Watching internal/graphs/ for changes..."
	@while true; do \
		inotifywait -q -e modify -r internal/graphs/ 2>/dev/null || fswatch -1 internal/graphs/ 2>/dev/null || sleep 5; \
		echo "Regenerating types..."; \
		$(MAKE) gen-types; \
	done

clean:
	rm -rf bin/ tmp/ coverage.out coverage.html

install-hooks:
	cp scripts/pre-commit.sh .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
