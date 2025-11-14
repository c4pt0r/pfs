.PHONY: build build-em build-wasi clean install-wasi install-em help

WASM_OUTPUT = hellofs-wasm-cpp.wasm
SRC = src/main.cpp
SDK_DIR = pfs-cpp-sdk

# Default target tries multiple compilers
build:
	@if command -v em++ >/dev/null 2>&1; then \
		$(MAKE) build-em; \
	elif [ -f /opt/wasi-sdk/bin/clang++ ]; then \
		$(MAKE) build-wasi; \
	else \
		echo "Error: No WASM compiler found!"; \
		echo "Please install one of:"; \
		echo "  - Emscripten: make install-em"; \
		echo "  - WASI SDK: make install-wasi"; \
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
	     -s ALLOW_MEMORY_GROWTH=0 \
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
	@echo "Building with WASI SDK..."
	/opt/wasi-sdk/bin/clang++ \
	    -std=c++17 \
	    -O3 \
	    -fno-exceptions \
	    -I$(SDK_DIR) \
	    --sysroot=/opt/wasi-sdk/share/wasi-sysroot \
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
	@if [ "$$(uname -m)" = "arm64" ]; then \
		curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-macos.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
	else \
		curl -L "https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-21/wasi-sdk-21.0-macos.tar.gz" -o /tmp/wasi-sdk.tar.gz; \
	fi
	@echo "Extracting..."
	@sudo tar xzf /tmp/wasi-sdk.tar.gz -C /opt
	@sudo mv /opt/wasi-sdk-21.0 /opt/wasi-sdk
	@echo "Cleaning up..."
	@rm /tmp/wasi-sdk.tar.gz
	@echo "WASI SDK installed to /opt/wasi-sdk"

clean:
	rm -f $(WASM_OUTPUT)

help:
	@echo "Available targets:"
	@echo "  make build  - Build the WASM plugin"
	@echo "  make clean  - Clean build artifacts"
	@echo ""
	@echo "Requirements:"
	@echo "  - clang++ with wasm32 target support"
