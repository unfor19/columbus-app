deps:
	go mod download -json

build:
	go build
	@ls -lh columbus-app*

run:
	go run .

test:
	go test ./... -v
