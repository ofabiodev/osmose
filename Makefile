.PHONY: generate fmt test race vet check build clean

generate:
	go generate ./...

fmt:
	go fmt ./...

test: generate
	go test ./...

race: generate
	go test -race ./...

vet: generate
	go vet ./...

check: fmt generate
	go test ./...
	go test -race ./...
	go vet ./...

build: generate
	go build -trimpath ./...

ifeq ($(OS),Windows_NT)
REMOVE_BIN := powershell -NoProfile -Command "if (Test-Path -LiteralPath 'bin') { Remove-Item -LiteralPath 'bin' -Recurse -Force }"
else
REMOVE_BIN := rm -rf bin
endif

clean:
	$(REMOVE_BIN)
