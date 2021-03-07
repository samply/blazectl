#!/usr/bin/env bash

VERSION=0.6.0

mkdir -p builds

GOOS=linux   GOARCH=amd64  go build
tar czf builds/blazectl-${VERSION}-linux-amd64.tar.gz blazectl
rm blazectl

GOOS=darwin  GOARCH=amd64  go build
tar czf builds/blazectl-${VERSION}-darwin-amd64.tar.gz blazectl
rm blazectl

GOOS=windows GOARCH=amd64  go build
zip -q builds/blazectl-${VERSION}-windows-amd64.zip blazectl.exe
rm blazectl.exe
