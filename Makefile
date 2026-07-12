BIN := sinty-timedate
PREFIX ?= /usr

.PHONY: all build test vet install clean

all: build

build:
	go build -o $(BIN) ./cmd/sinty-timedate

# Static, stripped binary for the OS image.
static:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/sinty-timedate

test:
	go test ./...

vet:
	go vet ./...

install: static
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)
