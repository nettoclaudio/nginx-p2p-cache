all: test

test: lint
	go test -race ./...

lint:
	golangci-lint run ./...
