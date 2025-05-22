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

## Test

```sh
go test ./...
```

## Build Releases

```sh
VERSION=1.0.0 ./build-releases.sh
```