.PHONY: build install clean uninstall test help

# Installation directory (can be overridden)
INSTALL_DIR ?= $(HOME)/.local/agfs-shell2
BIN_LINK_DIR ?= $(HOME)/.local/bin

help:
	@echo "agfs-shell2 build and installation"
	@echo ""
	@echo "Available targets:"
	@echo "  make build      - Build portable distribution with uv"
	@echo "  make install    - Install to $(INSTALL_DIR)"
	@echo "  make uninstall  - Remove installation"
	@echo "  make test       - Run tests with pytest"
	@echo "  make clean      - Clean build artifacts"
	@echo ""
	@echo "Override installation directory:"
	@echo "  make install INSTALL_DIR=/opt/agfs-shell2"
	@echo ""
	@echo "Requirements:"
	@echo "  - Python 3.8+"
	@echo "  - uv package manager"

build:
	@echo "Building portable agfs-shell2 distribution..."
	@python3 build.py

test:
	@echo "Running tests with pytest..."
	@uv run pytest tests/

install: build
	@echo "Installing agfs-shell2 to $(INSTALL_DIR)..."
	@rm -rf $(INSTALL_DIR)
	@mkdir -p $(INSTALL_DIR)
	@cp -r dist/agfs-shell2-portable/* $(INSTALL_DIR)/
	@mkdir -p $(BIN_LINK_DIR)
	@ln -sf $(INSTALL_DIR)/agfs-shell2 $(BIN_LINK_DIR)/agfs-shell2
	@echo "✓ Installed successfully"
	@echo "  Install dir: $(INSTALL_DIR)"
	@echo "  Symlink: $(BIN_LINK_DIR)/agfs-shell2"
	@echo ""
	@echo "Run 'agfs-shell2 --help' to get started"

uninstall:
	@echo "Removing agfs-shell2 installation..."
	@rm -rf $(INSTALL_DIR)
	@rm -f $(BIN_LINK_DIR)/agfs-shell2
	@echo "✓ Uninstalled successfully"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf build dist *.spec
	@echo "✓ Clean complete"
