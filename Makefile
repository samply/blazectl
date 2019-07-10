.PHONY: build, clean

build:
	GOOS=linux   GOARCH=amd64  go build -o builds/linux-amd64/blazectl
	GOOS=darwin  GOARCH=amd64  go build -o builds/darwin-amd64/blazectl
	GOOS=windows GOARCH=amd64  go build -o builds/windows-amd64/blazectl.exe

clean:
	rm -r builds
