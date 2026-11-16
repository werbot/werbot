.PHONY: test lint

test:
	go test -count=1 ./...

lint:
	golangci-lint run ./...
