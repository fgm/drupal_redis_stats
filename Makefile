# Stays on Make, no value in moving to Just.
#
# A public repo also pays twice: contributors need a new tool, and CI,
# which pins every action to a SHA, gains unpinned supply-chain surface.

all: lint test build

BINDIR   := bin
COVERDIR := coverage
GO       := go
PACKAGES := ./...

# If _RAW_GOBIN is empty, take the first element in _RAW_GOPATH
# Replace Windows/UNIX directory separators to create a "make" list.
# Then take the first word and append /bin
_RAW_GOBIN  := $(shell $(GO) env GOBIN)
_RAW_GOPATH := $(shell $(GO) env GOPATH)
GOBIN := $(if $(_RAW_GOBIN),$(_RAW_GOBIN),$(firstword $(subst :, ,$(subst ;, ,$(_RAW_GOPATH))))/bin)

.PHONY: all bench build clean cover install lint modernize test

# goimports -l exits 0 even when it lists files, so the output decides.
# It also formats as gofmt does, so a separate gofmt check adds nothing.
lint: modernize
	@out=$$($(GO) tool goimports -l .); test -z "$$out" || { echo "$$out"; exit 1; }
	$(GO) tool staticcheck -checks=all $(PACKAGES)

# go fix exits non-zero when fixes are pending, and prints them as a patch,
# so a failure here already says what to apply.
modernize:
	$(GO) fix -diff $(PACKAGES)

# -run matches no test, so only the benchmarks run.
bench:
	$(GO) test -run='^$$' -bench=. -benchmem $(PACKAGES)

cover:
	mkdir -p $(COVERDIR)
	# This runs the benchmarks just once, as unit tests, for coverage reporting only.
	# It does not replace running "make bench".
	$(GO) test -v -race -bench=. -benchtime=1x -coverprofile=$(COVERDIR)/cover.out -covermode=atomic $(PACKAGES)
	$(GO) tool cover -html=$(COVERDIR)/cover.out

test:
	# This includes any fuzz tests in unit test mode
	$(GO) test -race $(PACKAGES)

# go build discards its result when handed more than one package,
# so $(PACKAGES) would check that the command builds without ever producing it.
build:
	$(GO) build -o $(BINDIR)/ .

install: test build
	$(GO) install .

clean:
	@echo GOBIN: $(GOBIN)
	rm -fr $(COVERDIR)					\
		$(BINDIR)/drupal_redis_stats	\
		$(GOBIN)/drupal_redis_stats
