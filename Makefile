build:
	go build
	@ls -lh "$(PWD)/columbus-app"

run:
	go run .