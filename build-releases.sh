#!/usr/bin/env bash

mkdir -p builds

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
tar czf builds/blazectl-${VERSION}-linux-amd64.tar.gz blazectl
rm blazectl

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build
tar czf builds/blazectl-${VERSION}-linux-arm64.tar.gz blazectl
rm blazectl

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build
tar czf builds/blazectl-${VERSION}-darwin-amd64.tar.gz blazectl
rm blazectl

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build
tar czf builds/blazectl-${VERSION}-darwin-arm64.tar.gz blazectl
rm blazectl

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build
zip -q builds/blazectl-${VERSION}-windows-amd64.zip blazectl.exe
rm blazectl.exe
