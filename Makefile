MODULE  := github.com/Cepat-Kilat-Teknologi/snmp-olt-zte
APP     := snmp-olt-zte
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.buildTime=$(DATE)

.PHONY: tidy run build test vet fmt vulncheck docker clean help

help:          ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n",$$1,$$2}'

tidy:          ## resolve + tidy go.sum
	go mod tidy

run:           ## run locally
	go run ./cmd/api

build:         ## build static binary → bin/snmp-olt-zte
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(APP) ./cmd/api

test:          ## run tests
	go test ./...

vet:           ## static analysis
	go vet ./...

fmt:           ## format all .go files
	gofmt -w .

vulncheck:     ## run govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

docker:        ## build container image
	docker build --build-arg APP_VERSION=$(VERSION) --build-arg APP_COMMIT=$(COMMIT) --build-arg APP_BUILD_TIME=$(DATE) -t $(APP):$(VERSION) .

clean:         ## remove build artifacts
	rm -rf bin
