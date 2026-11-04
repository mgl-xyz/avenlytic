BINARY_DIR=dist
LDFLAGS=-trimpath -ldflags="-s -w"
.PHONY: test test-race build build-agent build-server build-web web-dev dev docker-build docker-up clean fmt vet

test:
	go test ./...
test-race:
	go test -race ./...
build-web:
	cd web-ui && npm install --no-audit --no-fund && npm run build
web-dev:
	cd web-ui && npm install --no-audit --no-fund && npm run dev

dev:
	@bash scripts/dev.sh
docker-build:
	docker build -t avenlytic:local .
docker-up:
	docker compose up -d --build
fmt:
	gofmt -w cmd internal
vet:
	go vet ./...
build: build-agent build-server
build-agent:
	mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-linux-amd64 ./cmd/agent
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-linux-arm64 ./cmd/agent
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-windows-amd64.exe ./cmd/agent
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-windows-arm64.exe ./cmd/agent
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-darwin-amd64 ./cmd/agent
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-agent-darwin-arm64 ./cmd/agent
build-server: build-web
	mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(BINARY_DIR)/avenlytic-server ./cmd/server
clean:
	rm -rf $(BINARY_DIR)
