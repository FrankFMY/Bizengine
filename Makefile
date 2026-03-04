.PHONY: build run dev test test-integration test-coverage migrate-up migrate-down lint clean

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

clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
