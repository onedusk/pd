.PHONY: build test test-e2e lint vet clean release-snapshot update-golden

build:
	CGO_ENABLED=1 go build -o bin/decompose ./cmd/decompose

test:
	CGO_ENABLED=1 go test -race -count=1 ./...

test-e2e:
	CGO_ENABLED=1 go test -tags e2e -race ./internal/e2e/

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/

release-snapshot:
	goreleaser build --snapshot --clean

update-golden:
	CGO_ENABLED=1 go test -tags e2e -run TestUpdateGolden ./internal/e2e/ -update
