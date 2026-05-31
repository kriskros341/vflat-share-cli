BINARY  ?= vflat
LDFLAGS := -s -w

.PHONY: build debug windows macos linux all tidy run clean

# Stripped binary for the current platform (default).
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

# Unstripped build (keeps symbols for debugging).
debug:
	go build -o $(BINARY) .

windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY).exe .

macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-macos .

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux .

# Cross-compile for all three platforms.
all: windows macos linux

tidy:
	go mod tidy

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) $(BINARY).exe $(BINARY)-macos $(BINARY)-linux
