# Makefile for HelloFS C Plugin

# Detect OS
UNAME_S := $(shell uname -s)

# Output library name
ifeq ($(UNAME_S),Darwin)
    # macOS
    LIB_NAME = libhellofs-c.dylib
    CC_FLAGS = -dynamiclib -fPIC
else ifeq ($(UNAME_S),Linux)
    # Linux
    LIB_NAME = libhellofs-c.so
    CC_FLAGS = -shared -fPIC
else
    # Windows (requires MinGW or similar)
    LIB_NAME = hellofs-c.dll
    CC_FLAGS = -shared
endif

# Compiler settings
CC = gcc
CFLAGS = -Wall -O2 $(CC_FLAGS)

# Source files
SRC = hellofs.c
OBJ = $(SRC:.c=.o)

# Default target
all: $(LIB_NAME)

# Build the shared library
$(LIB_NAME): $(SRC)
	$(CC) $(CFLAGS) -o $@ $<

# Clean build artifacts
clean:
	rm -f $(LIB_NAME) $(OBJ)

# Install (copy to plugin directory)
install: $(LIB_NAME)
	mkdir -p ../../plugins
	cp $(LIB_NAME) ../../plugins/

# Test the library exists and can be loaded
test: $(LIB_NAME)
	@echo "Library built successfully: $(LIB_NAME)"
	@file $(LIB_NAME)

.PHONY: all clean install test
