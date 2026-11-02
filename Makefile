.PHONY: test build package

test:
	cd src/web && bun run --bun test
	go test -race ./...

build:
	cd src/web && bun run --bun build
	mkdir -p build/worker
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-trimpath -tags lambda.norpc -o build/bootstrap ./src/api/cmd/server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-trimpath -tags lambda.norpc -o build/worker/bootstrap ./src/worker/cmd/worker

package: build
	python3 script/package.py
