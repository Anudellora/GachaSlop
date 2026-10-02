.PHONY: run test vet build backend-build frontend-install frontend-dev frontend-build frontend-test

run:
	go run ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

build: frontend-build backend-build

backend-build:
	mkdir -p bin
	go build -trimpath -ldflags='-s -w' -o bin/gachaslop-api ./cmd/api

frontend-install:
	npm --prefix web ci

frontend-dev:
	npm --prefix web run dev

frontend-build:
	npm --prefix web run build

frontend-test:
	npm --prefix web test
