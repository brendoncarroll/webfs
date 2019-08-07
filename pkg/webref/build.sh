#!/bin/sh
set -ve
protoc -I . --go_out=paths=source_relative:. ref.proto