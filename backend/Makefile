# Compiled binaries go to ./out (git- and docker-ignored), never into the source tree.
OUT := out

.PHONY: build build-server build-compiler build-parse run run-compiler test clean

build: build-server build-compiler build-parse

build-server:
	go build -o $(OUT)/server ./cmd/server

build-compiler:
	go build -o $(OUT)/compiler ./cmd/compiler

build-parse:
	go build -o $(OUT)/parse ./cmd/parse

# Compile and run in one step, without leaving a binary behind.
run:
	go run ./cmd/server

# Runs the compiler service locally (needs latexmk, prlimit and pdfinfo on PATH;
# prefer `docker compose --profile split up compiler`).
run-compiler:
	COMPILER_TOKEN=$${COMPILER_TOKEN:-dev-compiler-token} PORT=8081 go run ./cmd/compiler

test:
	go test ./... -count=1

clean:
	rm -rf $(OUT)
