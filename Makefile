.PHONY: test vet build demo

test:
	go test ./...
vet:
	go vet ./...
build:
	go build -o revenueleak ./cmd/server
demo:
	DEMO_MODE=true go run ./cmd/server
