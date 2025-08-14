lint:
	golangci-lint run

test:
	GOEXPERIMENT=jsonv2 go test ./...

build:
	GOEXPERIMENT=jsonv2 go build .

.PHONY: lint test build
