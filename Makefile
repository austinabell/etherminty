ETHERMINTY_DAEMON_BINARY = emtyd
ETHERMINTY_CLI_BINARY = emtycli
BUILD_FLAGS = -tags netgo -ldflags "-X github.com/cosmos/ethermint/version.GitCommit=${COMMIT_HASH}"
GO_MOD=GO111MODULE=on


build:
ifeq ($(OS),Windows_NT)
	${GO_MOD} go build $(BUILD_FLAGS) -o build/$(ETHERMINTY_DAEMON_BINARY).exe ./cmd/emtyd
	${GO_MOD} go build $(BUILD_FLAGS) -o build/$(ETHERMINTY_CLI_BINARY).exe ./cmd/emtycli
else
	${GO_MOD} go build $(BUILD_FLAGS) -o build/$(ETHERMINTY_DAEMON_BINARY) ./cmd/emtyd/
	${GO_MOD} go build $(BUILD_FLAGS) -o build/$(ETHERMINTY_CLI_BINARY) ./cmd/emtycli/
endif

install:
	${GO_MOD} go install $(BUILD_FLAGS) ./cmd/emtyd
	${GO_MOD} go install $(BUILD_FLAGS) ./cmd/emtycli

clean:
	@rm -rf ./build

verify:
	@echo "--> Verifying dependencies have not been modified"
	${GO_MOD} go mod verify