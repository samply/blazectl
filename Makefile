lint:
	golangci-lint run

test:
	go test ./...

vuln:
	go tool govulncheck ./...

build:
	go build .

.PHONY: lint test vuln build
