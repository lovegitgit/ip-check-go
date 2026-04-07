GO ?= go
BIN_DIR ?= bin
OUT_DIR ?= $(BIN_DIR)
CMDS := ip-check ip-check-cfg igeo-info igeo-dl igeo-cfg ip-filter
EXAMPLES := config-ex.ini geo-ex.ini
PLATFORMS := linux/amd64 linux/arm64 windows/amd64
SUPPORTED_GOOS := linux windows
SUPPORTED_GOARCH := amd64 arm64

.PHONY: all build tidy clean run help examples build-all build-platform

all: build

build: tidy
	@mkdir -p "$(OUT_DIR)"
	@for cmd in $(CMDS); do \
		ext=""; \
		target_os="$(GOOS)"; \
		target_arch="$(GOARCH)"; \
		if [ "$$target_os" = "windows" ]; then ext=".exe"; fi; \
		if [ -n "$$target_os" ] || [ -n "$$target_arch" ]; then \
			echo "building $$cmd for $${target_os:-host}/$${target_arch:-host}"; \
			GOOS="$$target_os" GOARCH="$$target_arch" $(GO) build -o "$(OUT_DIR)/$$cmd$$ext" "./cmd/$$cmd" || exit 1; \
		else \
			echo "building $$cmd"; \
			$(GO) build -o "$(OUT_DIR)/$$cmd" "./cmd/$$cmd" || exit 1; \
		fi; \
	done
	@$(MAKE) examples OUT_DIR="$(OUT_DIR)"

build-all: tidy
	@for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		outdir="$(BIN_DIR)/$${goos}-$${goarch}"; \
		$(MAKE) build GOOS="$$goos" GOARCH="$$goarch" OUT_DIR="$$outdir" || exit 1; \
	done

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)

examples:
	@mkdir -p "$(OUT_DIR)"
	@for file in $(EXAMPLES); do \
		echo "copying $$file"; \
		cp "$$file" "$(OUT_DIR)/$$file" || exit 1; \
	done

run:
	@echo "Usage: make build"
	@echo "Then run one of:"
	@for cmd in $(CMDS); do echo "  ./$(BIN_DIR)/$$cmd"; done

help:
	@echo "Targets:"
	@echo "  make build"
	@echo "      Build all commands for the current host into $(OUT_DIR)/"
	@echo "  make build GOOS=linux GOARCH=arm64 OUT_DIR=dist/linux-arm64"
	@echo "      Build all commands for a specific target into OUT_DIR"
	@echo "  make build-all"
	@echo "      Build all commands for: $(PLATFORMS)"
	@echo "  make examples OUT_DIR=dist"
	@echo "      Copy config-ex.ini and geo-ex.ini into OUT_DIR"
	@echo "  make tidy    Run go mod tidy"
	@echo "  make clean   Remove $(BIN_DIR)/"
	@echo "  make run     Show runnable binaries"
	@echo ""
	@echo "Requirements:"
	@echo "  Go version: 1.24+"
	@echo ""
	@echo "Variables:"
	@echo "  GOOS     Supported values: $(SUPPORTED_GOOS) (default: current host)"
	@echo "  GOARCH   Supported values: $(SUPPORTED_GOARCH) (default: current host)"
	@echo "  OUT_DIR  Output directory (default: $(BIN_DIR))"
	@echo ""
	@echo "Notice:"
	@echo "  This repository is a vibe coding artifact and has not been strictly tested."
	@echo "  Use it at your own risk."

$(BIN_DIR):
	mkdir -p $(BIN_DIR)
