all: clean lint test build

.PHONY: build
build: main.go cmd output redis stats
	@echo Building
	@go build -o drupal_redis_stats

.PHONY: clean
clean:
	@echo Cleaning
	@rm -f coverage* drupal_redis_stats

.PHONY: install
install: main.go cmd/credentials.go cmd/wire.go cmd/wire_gen.go output redis stats
	@echo Installing
	@go install

.PHONY: lint
lint: cmd/wire_gen.go
	@echo Linting
	@find . -type f -name "*.go" | xargs gofmt -l
	@find . -type f -name "*.go" | xargs goimports -l
	@golint ./...
	@staticcheck ./...

.PHONY: test
test: cmd output redis stats cmd/wire_gen.go
	@echo Testing
	@go test -race ./...

cmd/wire_gen.go: cmd/wire.go redis stats
	@echo Wiring
	@go generate ./...
