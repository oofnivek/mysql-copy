.PHONY: run build test tidy lint clean

BIN  := ./bin/server
CMD  := ./cmd/server
PORT ?= 8080

run:
	-lsof -ti :$(PORT) | xargs kill -9 2>/dev/null; true
	go run $(CMD)/...

build:
	go build -o $(BIN) $(CMD)/...

test:
	go test ./... -race -count=1

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

clean:
	rm -rf ./bin
