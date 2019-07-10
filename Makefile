.PHONY: build, clean

build:
	mkdir builds
	GOOS=linux   GOARCH=amd64  go build
	tar czf builds/blazectl-0.1.0-linux-amd64.tar.gz blazectl
	GOOS=darwin  GOARCH=amd64  go build
	tar czf builds/blazectl-0.1.0-darwin-amd64.tar.gz blazectl
	GOOS=windows GOARCH=amd64  go build
	zip builds/blazectl-0.1.0-windows-amd64.zip blazectl.exe

clean:
	rm -r builds
