# Stays on Make, no value in moving to Just.
#
# A public repo also pays twice: contributors need a new tool, and CI,
# which pins every action to a SHA, gains unpinned supply-chain surface.

all: lint test build

BINDIR   := bin
COVERDIR := coverage
GO       := go
PACKAGES := ./...

.PHONY: all bench build clean cover install lint modernize test

# goimports -l exits 0 even when it lists files, so the output decides,
# but a goimports failure, such as a syntax error, must fail the target too.
# It also formats as gofmt does, so a separate gofmt check adds nothing.
# Directories starting with _ are skipped, as the go tool does.
lint: modernize
	@out=$$(find . -name '_*' -prune -o -name '*.go' -print0 | xargs -0 $(GO) tool goimports -l) || exit 1; \
		test -z "$$out" || { echo "$$out"; exit 1; }
	$(GO) tool staticcheck -checks=all $(PACKAGES)

# go fix exits non-zero when fixes are pending, and prints them as a patch,
# so a failure here already says what to apply.
modernize:
	$(GO) fix -diff $(PACKAGES)

# -run matches no test, so only the benchmarks run.
bench:
	$(GO) test -run='^$$' -bench=. -benchmem $(PACKAGES)

# This runs the benchmarks just once, as unit tests, for coverage reporting only.
# It does not replace running "make bench".
cover:
	mkdir -p $(COVERDIR)
	$(GO) test -v -race -bench=. -benchtime=1x -coverprofile=$(COVERDIR)/cover.out -covermode=atomic $(PACKAGES)
	$(GO) tool cover -html=$(COVERDIR)/cover.out

# This includes any fuzz tests in unit test mode.
test:
	$(GO) test -race $(PACKAGES)

# go build discards its result when handed more than one package,
# so $(PACKAGES) would check that the command builds without ever producing it.
build:
	$(GO) build -o $(BINDIR)/ .

install: test build
	$(GO) install .

# The second path is where go install puts the command.
# Until Go 1.26, go env GOBIN is empty when GOBIN is unset,
# so $(go env GOBIN)/drupal_redis_stats would be /drupal_redis_stats:
# go list applies the go tool's own GOBIN and GOPATH rules, on every version.
clean:
	rm -fr "$(COVERDIR)"
	rm -f "$(BINDIR)/drupal_redis_stats" "$$($(GO) list -f '{{.Target}}' .)"
