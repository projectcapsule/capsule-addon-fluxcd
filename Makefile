SRC_ROOT = $(shell git rev-parse --show-toplevel)

## Location to install dependencies to
LOCALBIN ?= $(SRC_ROOT)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

GINKGO         ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT  ?= $(LOCALBIN)/golangci-lint

.PHONY: build
build:
	@go build .

.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) run -c .golangci.yml

.PHONY: e2e
e2e: ginkgo
	@$(GINKGO) -v -tags e2e $(SRC_ROOT)/e2e

.PHONY: e2e/charts
e2e/charts: ginkgo
	@$(GINKGO) -v -tags e2e $(SRC_ROOT)/e2e/charts

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(LOCALBIN)/ginkgo || GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo

helm-lint: CT_VERSION := v3.3.1
helm-lint: docker
	@docker run -v "$(SRC_ROOT):/workdir" --entrypoint /bin/sh quay.io/helmpack/chart-testing:$(CT_VERSION) -c "cd /workdir; ct lint --config .github/configs/ct.yaml  --lint-conf .github/configs/lintconf.yaml  --all --debug"

.PHONY: helm-docs
helm-docs: HELMDOCS_VERSION := v1.12.0
helm-docs: docker
	@docker run -v "$(SRC_ROOT):/helm-docs" jnorwood/helm-docs:$(HELMDOCS_VERSION) --chart-search-root=/helm-docs

.PHONY: docker
docker:
	@hash docker 2>/dev/null || {\
		echo "You need docker" &&\
		exit 1;\
	}
