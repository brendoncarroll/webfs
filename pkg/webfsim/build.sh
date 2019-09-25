protoc -I ../webref -I ../wrds -I ../cells/rwacryptocell -I . --go_out=paths=source_relative:. ./*.proto
