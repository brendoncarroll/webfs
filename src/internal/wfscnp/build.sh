#!/bin/sh

set -ve

capnp compile -I$(go list -m -f '{{.Dir}}' capnproto.org/go/capnp/v3)/std -ogo webfs.capnp
