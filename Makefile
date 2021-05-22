.PHONY: lint
lint:
	find . -type f -name *.go | xargs gofmt -l
	find . -type f -name *.go | xargs goimports -l
	staticcheck ./...
