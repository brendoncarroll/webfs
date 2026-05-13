
test: capnp
	go test ./...

testv:
	go test -v ./...

install: capnp
	go install ./cmd/webfs

install-capnpc-go:
	GOBIN="${GOBIN:-$(go env GOPATH)/bin}" go install capnproto.org/go/capnp/v3/capnpc-go@latest

capnp: install-capnpc-go
	PATH="$(go env GOPATH)/bin:${PATH}"; cd ./src/internal/wfscnp && ./build.sh
