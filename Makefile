lint:
	golangci-lint run

test:
	GOEXPERIMENT=jsonv2 go test ./...

vuln:
	GOEXPERIMENT=jsonv2 go tool govulncheck ./...

build:
	GOEXPERIMENT=jsonv2 go build .

.PHONY: lint test vuln build
