deps:
	go mod download -json

build:
	go build
	@ls -lh "$(PWD)/columbus-app"

run:
	go run .