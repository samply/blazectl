lint:
	golangci-lint run

test:
	go test ./...

.PHONY: lint test
