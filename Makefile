GINKGO ?= $(shell command -v ginkgo)
GOLANGCI_LINT ?= $(shell command -v golangci-lint)

.PHONY: build
build:
	@go build .

.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) run -c .golangci.yml

.PHONY: e2e
e2e: ginkgo
	@$(GINKGO) -v -tags e2e ./e2e

.PHONY: e2e/charts
e2e/charts: ginkgo
	$(GINKGO) -v -tags e2e ./e2e/charts

.PHONY: golangci-lint
golangci-lint:
	@hash ginkgo 2>/dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

.PHONY: ginkgo
ginkgo:
	@hash ginkgo 2>/dev/null || go install github.com/onsi/ginkgo/v2/ginkgo@v2.13.2
