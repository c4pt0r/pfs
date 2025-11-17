.PHONY: build install clean uninstall help

# Installation directory (can be overridden)
INSTALL_DIR ?= $(HOME)/.local/agfs
BIN_LINK_DIR ?= $(HOME)/.local/bin

help:
	@echo "agfs-shell build and installation"
	@echo ""
	@echo "Available targets:"
	@echo "  make build      - Build portable distribution with uv"
	@echo "  make install    - Install to $(INSTALL_DIR)"
	@echo "  make uninstall  - Remove installation"
	@echo "  make clean      - Clean build artifacts"
	@echo ""
	@echo "Override installation directory:"
	@echo "  make install INSTALL_DIR=/opt/agfs"
	@echo ""
	@echo "Requirements:"
	@echo "  - Python 3.10+"
	@echo "  - uv package manager"

build:
	@echo "Building portable agfs distribution..."
	@python3 build.py

install: build
	@echo "Installing agfs to $(INSTALL_DIR)..."
	@rm -rf $(INSTALL_DIR)
	@mkdir -p $(INSTALL_DIR)
	@cp -r dist/agfs-portable/* $(INSTALL_DIR)/
	@mkdir -p $(BIN_LINK_DIR)
	@ln -sf $(INSTALL_DIR)/agfs $(BIN_LINK_DIR)/agfs
	@echo "✓ Installed successfully"
	@echo "  Install dir: $(INSTALL_DIR)"
	@echo "  Symlink: $(BIN_LINK_DIR)/agfs"
	@echo ""
	@echo "Run 'agfs --help' to get started"

uninstall:
	@echo "Removing agfs installation..."
	@rm -rf $(INSTALL_DIR)
	@rm -f $(BIN_LINK_DIR)/agfs
	@echo "✓ Uninstalled successfully"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf build dist *.spec agfs_entry.py
	@echo "✓ Clean complete"
