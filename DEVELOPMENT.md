# Development

## Update all Dependencies

```sh
go get -u ./...
```

## Update a Dependency

```sh
go get <dependency name>
```

### Example

```sh
go get github.com/vbauerster/mpb/v7
```

## Lint

```sh
golangci-lint run
```

## Test

```sh
go test ./...
```
