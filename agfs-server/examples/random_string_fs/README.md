# random_string_fs

A WASM filesystem plugin that generates random alphanumeric strings.

## Features

- Generates random strings containing characters from `[a-zA-Z0-9]`
- Configurable length (1-1024 characters)
- Simple read/write interface

## Building

```bash
make
```

This will compile the plugin to `random_string_fs.wasm`.

## Loading

```bash
agfs plugins load ./random_string_fs.wasm
```

## Usage

### Generate random string with default length (6 characters)

```bash
cat /agfs/random_string_fs/generate
```

### Generate random string with custom length

Write the desired length, then read:

```bash
echo "16" > /agfs/random_string_fs/generate
cat /agfs/random_string_fs/generate
```

Or in one command (write returns the generated string):

```bash
echo "16" > /agfs/random_string_fs/generate
```

## Limitations

- Length must be between 1 and 1024 characters
- Only alphanumeric characters (a-z, A-Z, 0-9)
- Read-only filesystem (cannot create/delete files)
