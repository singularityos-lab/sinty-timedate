BIN := sinty-timedate
DAEMON := sinty-timedated
PREFIX ?= /usr

.PHONY: all build test vet install clean

all: build

build:
	go build -o $(BIN) ./cmd/sinty-timedate
	go build -o $(DAEMON) ./cmd/sinty-timedated

release:
	go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/sinty-timedate
	go build -trimpath -ldflags "-s -w" -o $(DAEMON) ./cmd/sinty-timedated

test:
	go test ./...

vet:
	go vet ./...

install: release
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/bin/$(BIN)
	install -Dm755 $(DAEMON) $(DESTDIR)$(PREFIX)/libexec/$(DAEMON)
	install -Dm644 packaging/org.freedesktop.timedate1.conf $(DESTDIR)$(PREFIX)/share/dbus-1/system.d/org.freedesktop.timedate1.conf
	install -Dm644 packaging/org.freedesktop.timedate1.service $(DESTDIR)$(PREFIX)/share/dbus-1/system-services/org.freedesktop.timedate1.service
	install -Dm644 packaging/sinty-timedated.service $(DESTDIR)$(PREFIX)/lib/systemd/system/sinty-timedated.service

clean:
	rm -f $(BIN) $(DAEMON)
