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
VERSION=0.17.1 ./build-releases.sh
```