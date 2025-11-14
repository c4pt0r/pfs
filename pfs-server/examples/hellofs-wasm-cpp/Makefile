.PHONY: build build-em build-wasi clean install-wasi install-wasi-local install-em help

WASM_OUTPUT = hellofs-wasm-cpp.wasm
SRC = src/main.cpp
SDK_DIR = pfs-cpp-sdk

# WASI SDK path (can be overridden with WASI_SDK_PATH environment variable)
WASI_SDK_PATH ?= /opt/wasi-sdk
LOCAL_WASI_SDK = $(HOME)/.local/wasi-sdk

# Default target tries multiple compilers
build:
	@if command -v em++ >/dev/null 2>&1; then \
		$(MAKE) build-em; \
	elif [ -f $(LOCAL_WASI_SDK)/bin/clang++ ]; then \
		WASI_SDK_PATH=$(LOCAL_WASI_SDK) $(MAKE) build-wasi; \
	elif [ -f $(WASI_SDK_PATH)/bin/clang++ ]; then \
		$(MAKE) build-wasi; \
	else \
		echo "Error: No WASM compiler found!"; \
		echo "Please install one of:"; \
		echo "  - Emscripten: make install-em"; \
		echo "  - WASI SDK (system): make install-wasi (requires sudo)"; \
		echo "  - WASI SDK (local): make install-wasi-local (no sudo)"; \
		exit 1; \
	fi

# Build with Emscripten
build-em:
	@echo "Building with Emscripten..."
	em++ -std=c++17 \
	     -O3 \
	     -fno-exceptions \
	     -fno-rtti \
	     -I$(SDK_DIR) \
	     -s WASM=1 \
	     -s STANDALONE_WASM=1 \
	     -s ALLOW_MEMORY_GROWTH=1 \
	     -s INITIAL_MEMORY=2MB \
	     -s MAXIMUM_MEMORY=16MB \
	     -s EXPORTED_FUNCTIONS='["_malloc","_free"]' \
	     -s ERROR_ON_UNDEFINED_SYMBOLS=0 \
	     --no-entry \
	     $(SRC) -o $(WASM_OUTPUT)
	@echo "Build complete: $(WASM_OUTPUT)"
	@ls -lh $(WASM_OUTPUT)

# Build with WASI SDK
build-wasi:
	@echo "Building with WASI SDK at $(WASI_SDK_PATH)..."
	$(WASI_SDK_PATH)/bin/clang++ \
	    -std=c++17 \
	    -O3 \
	    -fno-exceptions \
	    -I$(SDK_DIR) \
	    --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot \
	    -Wl,--no-entry \
	    -Wl,--export-dynamic \
	    -Wl,--allow-undefined \
	    $(SRC) -o $(WASM_OUTPUT)
	@echo "Build complete: $(WASM_OUTPUT)"
	@ls -lh $(WASM_OUTPUT)

# Install Emscripten (macOS)
install-em:
	@echo "Installing Emscripten..."
	@if command -v brew >/dev/null 2>&1; then \
		brew install emscripten; \
	else \
		echo "Homebrew not found. Please install from: https://brew.sh/"; \
		exit 1; \
	fi

# Install WASI SDK
install-wasi:
	@echo "Installing WASI SDK..."
	@echo "Downloading WASI SDK..."
	@mkdir -p /tmp/wasi-sdk
	@OS=$$(uname -s); \
	ARCH=$$(uname -m); \
	if [ "$$OS" = "Darwin" ]; then \
		echo "Detected macOS"; \
		curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-macos.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
	elif [ "$$OS" = "Linux" ]; then \
		if [ "$$ARCH" = "x86_64" ]; then \
			echo "Detected Linux x86_64"; \
			curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-linux.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
		elif [ "$$ARCH" = "aarch64" ]; then \
			echo "Detected Linux aarch64"; \
			curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-24/wasi-sdk-24.0-arm64-linux.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
		else \
			echo "Unsupported architecture: $$ARCH"; \
			exit 1; \
		fi; \
	else \
		echo "Unsupported OS: $$OS"; \
		exit 1; \
	fi
	@echo "Extracting..."
	@sudo tar xzf /tmp/wasi-sdk.tar.gz -C /opt
	@if [ -d /opt/wasi-sdk-21.0 ]; then \
		sudo rm -rf /opt/wasi-sdk; \
		sudo mv /opt/wasi-sdk-21.0 /opt/wasi-sdk; \
	elif [ -d /opt/wasi-sdk-24.0-arm64-linux ]; then \
		sudo rm -rf /opt/wasi-sdk; \
		sudo mv /opt/wasi-sdk-24.0-arm64-linux /opt/wasi-sdk; \
	fi
	@echo "Cleaning up..."
	@rm /tmp/wasi-sdk.tar.gz
	@echo "WASI SDK installed to /opt/wasi-sdk"

# Install WASI SDK to user's home directory (no sudo required)
install-wasi-local:
	@echo "Installing WASI SDK to $(LOCAL_WASI_SDK)..."
	@echo "Downloading WASI SDK..."
	@mkdir -p /tmp/wasi-sdk
	@OS=$$(uname -s); \
	ARCH=$$(uname -m); \
	if [ "$$OS" = "Darwin" ]; then \
		echo "Detected macOS"; \
		curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-macos.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
		EXTRACTED_DIR="wasi-sdk-21.0"; \
	elif [ "$$OS" = "Linux" ]; then \
		if [ "$$ARCH" = "x86_64" ]; then \
			echo "Detected Linux x86_64"; \
			curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-linux.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
			EXTRACTED_DIR="wasi-sdk-21.0"; \
		elif [ "$$ARCH" = "aarch64" ]; then \
			echo "Detected Linux aarch64"; \
			curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-24/wasi-sdk-24.0-arm64-linux.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
			EXTRACTED_DIR="wasi-sdk-24.0-arm64-linux"; \
		else \
			echo "Unsupported architecture: $$ARCH"; \
			exit 1; \
		fi; \
	else \
		echo "Unsupported OS: $$OS"; \
		exit 1; \
	fi; \
	echo "Extracting..."; \
	tar xzf /tmp/wasi-sdk.tar.gz -C /tmp/wasi-sdk; \
	mkdir -p $(HOME)/.local; \
	rm -rf $(LOCAL_WASI_SDK); \
	mv /tmp/wasi-sdk/$$EXTRACTED_DIR $(LOCAL_WASI_SDK); \
	echo "Cleaning up..."; \
	rm /tmp/wasi-sdk.tar.gz; \
	rm -rf /tmp/wasi-sdk; \
	echo "WASI SDK installed to $(LOCAL_WASI_SDK)"

clean:
	rm -f $(WASM_OUTPUT)

help:
	@echo "Available targets:"
	@echo "  make build  - Build the WASM plugin"
	@echo "  make clean  - Clean build artifacts"
	@echo ""
	@echo "Requirements:"
	@echo "  - clang++ with wasm32 target support"
