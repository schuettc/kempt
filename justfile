version := `cat VERSION`
commit  := `git rev-parse --short HEAD 2>/dev/null || echo none`
date    := `date -u +%Y-%m-%d`
ldflags := "-X github.com/schuettc/kempt/internal/version.version=" + version + " -X github.com/schuettc/kempt/internal/version.commit=" + commit + " -X github.com/schuettc/kempt/internal/version.date=" + date

default: verify

fmt-check:
    test -z "$(gofmt -l .)"

lint:
    go vet ./...

test:
    go test ./...

build:
    go build -ldflags '{{ldflags}}' -o kempt ./cmd/kempt

cross:
    GOOS=darwin GOARCH=arm64 go build -ldflags '{{ldflags}}' -o /dev/null ./cmd/kempt
    GOOS=darwin GOARCH=amd64 go build -ldflags '{{ldflags}}' -o /dev/null ./cmd/kempt
    GOOS=linux  GOARCH=amd64 go build -ldflags '{{ldflags}}' -o /dev/null ./cmd/kempt
    GOOS=linux  GOARCH=arm64 go build -ldflags '{{ldflags}}' -o /dev/null ./cmd/kempt

verify: fmt-check lint test build cross
