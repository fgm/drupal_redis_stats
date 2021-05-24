all: clean lint test build

.PHONY: build
build: main.go redis.go credentials.go wire.go wire_gen.go output stats
	@echo Building
	@go build -o drupal_redis_stats

.PHONY: clean
clean:
	@echo Cleaning
	@rm -f coverage* drupal_redis_stats

.PHONY: install
install: main.go redis.go credentials.go wire.go wire_gen.go output stats
	@echo Installing
	@go install

.PHONY: lint
lint: wire_gen.go
	@echo Linting
	@find . -type f -name "*.go" | xargs gofmt -l
	@find . -type f -name "*.go" | xargs goimports -l
	@golint ./...
	@staticcheck ./...

.PHONY: test
test: wire_gen.go
	@echo Testing
	@go test -race ./...

wire_gen.go: credentials.go redis.go wire.go
	@echo Wiring
	@go generate
