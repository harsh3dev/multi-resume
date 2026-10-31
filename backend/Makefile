# Compiled binaries go to ./out (git- and docker-ignored), never into the source tree.
OUT := out

.PHONY: build build-server build-parse run test clean

build: build-server build-parse

build-server:
	go build -o $(OUT)/server ./cmd/server

build-parse:
	go build -o $(OUT)/parse ./cmd/parse

# Compile and run in one step, without leaving a binary behind.
run:
	go run ./cmd/server

test:
	go test ./... -count=1

clean:
	rm -rf $(OUT)
