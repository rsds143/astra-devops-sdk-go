all: lint test

lint:
	golangci-lint run
test:
	go test -timeout=1200s -v ./...

