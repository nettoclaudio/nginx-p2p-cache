PROTOC ?= protoc

all: test

generate:
	$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/nginx/cache_repository/v1/*.proto

test: lint
	go test -race ./...

lint:
	golangci-lint run ./...
