#!/usr/bin/make -f

# Version stamp: closest git tag (without the leading "v"), or the short commit
# when there is no tag. Both can be overridden: make build VERSION=4.0.4
VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//')
COMMIT ?= $(shell git log -1 --format='%H' 2>/dev/null)
LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin
BUILDDIR ?= $(CURDIR)/build
SIMAPP = ./app
DOCKER := $(shell which docker)
DOCKER_IMAGE ?= passage
DOCKER_TAG ?= local

export GO111MODULE = on

###############################################################################
###                            Build flags                                  ###
###############################################################################

build_tags = netgo
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
      else
        build_tags += ledger
      endif
    endif
  endif
endif

ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  build_tags += gcc
endif

build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

empty :=
space := $(empty) $(empty)
comma := ,
build_tags_comma_sep := $(subst $(space),$(comma),$(build_tags))

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=passage \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=passage \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)"

# DB backend selection
ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif
ifeq (pebbledb,$(findstring pebbledb,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=pebbledb
endif

ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -w -s
endif
ifeq ($(LINK_STATICALLY),true)
  ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static"
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'
ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
endif

all: build lint test

###############################################################################
###                                  Build                                  ###
###############################################################################

BUILD_TARGETS := build install

build: BUILD_ARGS=-o $(BUILDDIR)/

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./cmd/passage

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

# Static linux binary (musl + libwasmvm) built in Docker, written to ./build/passage.
build-linux:
	$(DOCKER) build --target export --output type=local,dest=$(BUILDDIR) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) .

# Runtime image passage:local (used by the localnet).
docker-build:
	$(DOCKER) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) .

# Release binary + libwasmvm runtime bundle, as built by .github/workflows/release.yml
# (linux, needs Go 1.25.14 and patchelf). VERSION=v4.1.0-rc1 GOARCH=amd64 by default.
release-build:
	bash scripts/build-security-release.sh

.PHONY: build install build-linux docker-build release-build

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify

clean:
	rm -rf $(BUILDDIR)/ artifacts/ dist/

.PHONY: clean

###############################################################################
###                           Tests & Simulation                            ###
###############################################################################

TEST_PACKAGES=./...
TEST_TAGS='cgo ledger test_ledger_mock norace'

test: test-unit

test-unit:
	go test -mod=readonly -tags=$(TEST_TAGS) -timeout 30m $(TEST_PACKAGES)

test-race:
	go test -mod=readonly -race -tags='cgo ledger test_ledger_mock' -timeout 60m $(TEST_PACKAGES)

test-cover:
	go test -mod=readonly -tags=$(TEST_TAGS) -timeout 30m -coverprofile=coverage.txt -covermode=atomic $(TEST_PACKAGES)

# Passage-specific tests of the vendored forks (the full upstream suites are long).
test-forks:
	cd iavl-fork && go test -count=1 -run 'TestPassage' .
	go test -count=1 ./app/params/...

test-all: test-unit test-forks test-sim-import-export test-sim-after-import test-sim-nondeterminism

SIM_NUM_BLOCKS ?= 100
SIM_BLOCK_SIZE ?= 50
SIM_SEED ?= 42

test-sim-full:
	@echo "Running full application simulation..."
	go test -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=5 -v -timeout 2h

test-sim-import-export:
	@echo "Running application import/export simulation..."
	go test -mod=readonly $(SIMAPP) -run TestAppImportExport -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=5 -v -timeout 2h

test-sim-after-import:
	@echo "Running application simulation-after-import..."
	go test -mod=readonly $(SIMAPP) -run TestAppSimulationAfterImport -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=5 -v -timeout 2h

test-sim-nondeterminism:
	@echo "Running non-determinism simulation..."
	go test -mod=readonly $(SIMAPP) -run TestAppStateDeterminism -Enabled=true \
		-NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Period=0 -v -timeout 2h

test-sim-custom-genesis:
	@echo "Running simulation from a custom genesis (default: $(HOME)/.passage/config/genesis.json)..."
	go test -mod=readonly $(SIMAPP) -run TestFullAppSimulation -Genesis=$(HOME)/.passage/config/genesis.json \
		-Enabled=true -NumBlocks=$(SIM_NUM_BLOCKS) -BlockSize=$(SIM_BLOCK_SIZE) -Commit=true -Seed=$(SIM_SEED) -Period=5 -v -timeout 2h

# In-place upgrade v4.0.3 (mainnet) -> working tree, two nodes, in Docker.
test-upgrade:
	bash scripts/tests/upgrade_test.sh

# Passage contracts (passage-contracts artifacts, legacy excluded) and passage-2 contract code
# across the v4.0.3 -> working tree upgrade, in Docker. CONTRACTS_DIR / MAINNET_DIR / QUICK=1.
test-contracts:
	bash scripts/tests/contracts_test.sh

.PHONY: test test-unit test-race test-cover test-forks test-all \
	test-sim-full test-sim-import-export test-sim-after-import test-sim-nondeterminism \
	test-sim-custom-genesis test-upgrade test-contracts

###############################################################################
###                                Linting                                  ###
###############################################################################

golangci_lint_cmd=golangci-lint
golangci_version=v1.64.8

lint:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=15m

lint-fix:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --fix --issues-exit-code=0

format:
	@go install mvdan.cc/gofumpt@v0.6.0
	find . -name '*.go' -type f -not -path "./iavl-fork/*" -not -path "*.git*" \
		-not -name "*.pb.go" -not -name "*.pb.gw.go" | xargs gofumpt -w -l

.PHONY: lint lint-fix format

###############################################################################
###                                Protobuf                                 ###
###############################################################################

protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

proto-all: proto-format proto-lint proto-gen

proto-gen:
	@echo "Generating Protobuf files"
	@$(protoImage) sh ./scripts/protocgen.sh

proto-format:
	@$(protoImage) find ./proto -name "*.proto" -exec clang-format -i {} \;

proto-lint:
	@$(protoImage) sh -c "cd proto && buf lint --error-format=json"

# Breaking-change check of ./proto against a git input (default: the local main branch). CI passes
# the pull request base: PROTO_BREAKING_AGAINST='https://github.com/<repo>.git#branch=<base>,subdir=proto'
PROTO_BREAKING_AGAINST ?= .git\#branch=main,subdir=proto

proto-check-breaking:
	@$(protoImage) sh -c "cd proto && buf breaking --against '$(PROTO_BREAKING_AGAINST)'"

.PHONY: proto-all proto-gen proto-format proto-lint proto-check-breaking

###############################################################################
###                                Localnet                                 ###
###############################################################################

LOCALNET_DIR ?= $(CURDIR)/mytestnet
LOCALNET_CHAIN_ID ?= passage-localnet

# Start a 4-validator localnet (docker-compose.yml) from the passage:local image.
localnet-start: localnet-stop docker-build $(LOCALNET_DIR)
	$(DOCKER) compose up -d
	@echo "RPC: http://localhost:26657  REST: http://localhost:1317  gRPC: localhost:9090"

$(LOCALNET_DIR):
	$(DOCKER) run --rm -v $(LOCALNET_DIR):/mytestnet $(DOCKER_IMAGE):$(DOCKER_TAG) \
		passage testnet --v 4 -o /mytestnet --chain-id $(LOCALNET_CHAIN_ID) --keyring-backend test \
		--node-dir-prefix passagenode --node-daemon-home passage --starting-ip-address 192.168.10.2

localnet-stop:
	$(DOCKER) compose down

# Files in ./mytestnet are owned by root (created in containers): remove them from a container too.
localnet-clean: localnet-stop
	$(DOCKER) run --rm -v $(CURDIR):/work alpine rm -rf /work/mytestnet

localnet-logs:
	$(DOCKER) compose logs -f --tail=100

.PHONY: localnet-start localnet-stop localnet-clean localnet-logs
