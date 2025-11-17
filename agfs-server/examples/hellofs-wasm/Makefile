.PHONY: build clean install test

# WASM target
WASM_TARGET = wasm32-unknown-unknown
WASM_OUTPUT = target/$(WASM_TARGET)/release/hellofs_wasm.wasm
OPTIMIZED_OUTPUT = hellofs-wasm.wasm

build:
	@echo "Building hellofs-wasm plugin..."
	cargo build --release --target $(WASM_TARGET)
	@if command -v wasm-opt >/dev/null 2>&1; then \
		wasm-opt -Oz $(WASM_OUTPUT) -o $(OPTIMIZED_OUTPUT); \
		echo "Optimized WASM: $(OPTIMIZED_OUTPUT)"; \
	else \
		cp $(WASM_OUTPUT) $(OPTIMIZED_OUTPUT); \
	fi

clean:
	cargo clean
	rm -f $(OPTIMIZED_OUTPUT)

install:
	rustup target add $(WASM_TARGET)

test:
	@echo "Testing WASM plugin with agfs-server..."
	@echo "Make sure agfs-server is built first"

help:
	@echo "Available targets:"
	@echo "  make install  - Install WASM target for Rust"
	@echo "  make build    - Build the WASM plugin"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make test     - Test the plugin with agfs-server"
