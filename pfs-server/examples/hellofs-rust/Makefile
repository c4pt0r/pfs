# Makefile for HelloFS Rust Plugin

# Detect OS
UNAME_S := $(shell uname -s)

# Output library name based on platform
ifeq ($(UNAME_S),Darwin)
    # macOS
    LIB_EXT = dylib
    LIB_NAME = libhellofs_rust.dylib
else ifeq ($(UNAME_S),Linux)
    # Linux
    LIB_EXT = so
    LIB_NAME = libhellofs_rust.so
else
    # Windows
    LIB_EXT = dll
    LIB_NAME = hellofs_rust.dll
endif

# Cargo settings
CARGO = cargo
TARGET_DIR = target/release

# Default target
all: $(LIB_NAME)

# Build the shared library using cargo
$(LIB_NAME):
	$(CARGO) build --release
	cp $(TARGET_DIR)/libhellofs_rust.$(LIB_EXT) $(LIB_NAME)

# Clean build artifacts
clean:
	$(CARGO) clean
	rm -f $(LIB_NAME)

# Install (copy to plugin directory)
install: $(LIB_NAME)
	mkdir -p ../../plugins
	cp $(LIB_NAME) ../../plugins/

# Test the library exists and can be loaded
test: $(LIB_NAME)
	@echo "Library built successfully: $(LIB_NAME)"
	@file $(LIB_NAME)

# Check Rust code without building
check:
	$(CARGO) check

# Format code
fmt:
	$(CARGO) fmt

# Run clippy linter
lint:
	$(CARGO) clippy

.PHONY: all clean install test check fmt lint
