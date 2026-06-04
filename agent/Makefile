.PHONY: build build-all test lint clean

BIN := softsentry-agent
DIST := dist

build:
	go build -o $(DIST)/$(BIN) ./cmd/softsentry-agent

build-all: clean
	GOOS=windows GOARCH=amd64 go build -o $(DIST)/$(BIN)-windows-amd64.exe ./cmd/softsentry-agent
	GOOS=darwin  GOARCH=amd64 go build -o $(DIST)/$(BIN)-darwin-amd64  ./cmd/softsentry-agent
	GOOS=darwin  GOARCH=arm64 go build -o $(DIST)/$(BIN)-darwin-arm64  ./cmd/softsentry-agent

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(DIST)
