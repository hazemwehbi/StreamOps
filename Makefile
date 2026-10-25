.PHONY: test integration race vet fmt build run migrate up down load
test:
	go test ./...
integration:
	go test -count=1 -v ./tests/integration
race:
	go test -race ./...
vet:
	go vet ./...
fmt:
	gofmt -w cmd internal migrations tests
build:
	go build -trimpath -o bin/streamops ./cmd/api
run:
	go run ./cmd/api
migrate:
	go run ./cmd/api migrate
up:
	docker compose up --build -d
down:
	docker compose down
load:
	k6 run tests/load/scenario.js
