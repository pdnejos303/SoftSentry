.PHONY: build build-all manifest test lint clean

BIN     := softsentry-agent
DIST    := dist
# Bump this on every release. It is stamped into the binary AND written into the
# manifest, so the two can never disagree (a mismatch causes an agent restart
# loop — see internal/transport/client.go).
VERSION := 0.1.0
PKG     := github.com/softsentry/agent/internal/transport
LDFLAGS := -X $(PKG).Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN) ./cmd/softsentry-agent

build-all: clean
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)-$(VERSION)-windows-amd64.exe ./cmd/softsentry-agent
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)-$(VERSION)-darwin-amd64  ./cmd/softsentry-agent
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)-$(VERSION)-darwin-arm64  ./cmd/softsentry-agent

# Write dist/manifest.json describing every binary in build-all, with the SAME
# VERSION stamped into them and a real SHA-256, so the backend never offers an
# update the binary cannot satisfy. Copy dist/* into the backend's
# agent_binary_dir to publish.
manifest: build-all
	@printf '{\n  "binaries": [\n' > $(DIST)/manifest.json
	@printf '    {"version": "%s", "os": "windows", "arch": "amd64", "filename": "%s-%s-windows-amd64.exe", "sha256": "%s"},\n' \
		"$(VERSION)" "$(BIN)" "$(VERSION)" "$$(shasum -a 256 $(DIST)/$(BIN)-$(VERSION)-windows-amd64.exe | cut -d' ' -f1)" >> $(DIST)/manifest.json
	@printf '    {"version": "%s", "os": "darwin", "arch": "amd64", "filename": "%s-%s-darwin-amd64", "sha256": "%s"},\n' \
		"$(VERSION)" "$(BIN)" "$(VERSION)" "$$(shasum -a 256 $(DIST)/$(BIN)-$(VERSION)-darwin-amd64 | cut -d' ' -f1)" >> $(DIST)/manifest.json
	@printf '    {"version": "%s", "os": "darwin", "arch": "arm64", "filename": "%s-%s-darwin-arm64", "sha256": "%s"}\n' \
		"$(VERSION)" "$(BIN)" "$(VERSION)" "$$(shasum -a 256 $(DIST)/$(BIN)-$(VERSION)-darwin-arm64 | cut -d' ' -f1)" >> $(DIST)/manifest.json
	@printf '  ]\n}\n' >> $(DIST)/manifest.json
	@echo "wrote $(DIST)/manifest.json (version $(VERSION))"

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf $(DIST)
