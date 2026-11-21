SHELL := /bin/bash
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build build-ui build-daemon build-app run-daemon clean check

build:
	$(MAKE) build-ui
	$(MAKE) build-daemon
	$(MAKE) build-app

build-ui:
	cd frontend && npm install && npm run build

build-daemon:
	mkdir -p build/bin
	go build -ldflags "$(LDFLAGS)" -o build/bin/shikigamid ./cmd/shikigamid
	go build -ldflags "$(LDFLAGS)" -o build/bin/shikigamictl ./cmd/shikigamictl

build-app:
	mkdir -p build/bin app/dist
	test -d frontend/dist || { echo "frontend/dist belum ada; jalankan 'make build-ui' dulu" >&2; exit 1; }
	find app/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R frontend/dist/. app/dist/
	CGO_LDFLAGS="-framework UniformTypeIdentifiers" go build -tags desktop,production -o build/bin/shikigami ./app

run-daemon: build-daemon
	./build/bin/shikigamid

clean:
	rm -rf build frontend/dist
	find app/dist -mindepth 1 ! -name .gitkeep -delete

check:
	go build ./...
