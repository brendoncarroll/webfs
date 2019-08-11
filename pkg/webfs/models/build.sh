protoc -I ../../webref -I ../../wrds -I ../../cells/cryptocell -I . --go_out=paths=source_relative:. ./*.proto
