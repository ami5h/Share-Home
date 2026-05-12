.PHONY: init run build clean logs cli

init:
	@echo "Edit .env with your settings, then run 'make run'"

run:
	docker compose up --build -d

run-fg:
	docker compose up --build

build:
	docker compose build

clean:
	docker compose down -v
	docker compose rm -fsv

logs:
	docker compose logs -f share-home

cli:
	@mkdir -p bin
	@echo "Building CLI for $(shell uname -s)/$(shell uname -m)..."
	@docker run --rm -v $(PWD)/cli:/src -v $(PWD)/bin:/out -w /src \
		golang:1.24-alpine sh -c "apk add --no-cache git && \
		CGO_ENABLED=0 GOOS=$(shell uname -s | tr A-Z a-z) GOARCH=$(shell uname -m | sed 's/x86_64/amd64/') go build -o /out/share-home ."
