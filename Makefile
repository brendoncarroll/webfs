
.PHONY: test testv install

test:
	go test ./...

testv:
	go test -v ./...

install:
	go install ./cmd/webfs
