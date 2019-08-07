set -ve
protoc -I . -I ../webref --go_out=paths=source_relative:. tree.proto

