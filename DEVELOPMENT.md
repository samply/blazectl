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

## Build Releases

```sh
VERSION=0.13.0 ./build-releases.sh
```