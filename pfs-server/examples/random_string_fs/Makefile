.PHONY: all clean

TARGET = random_string_fs.wasm
CARGO_TARGET = wasm32-unknown-unknown

all: $(TARGET)

$(TARGET):
	cargo build --release --target $(CARGO_TARGET)
	cp target/$(CARGO_TARGET)/release/$(TARGET) .

clean:
	cargo clean
	rm -f $(TARGET)
