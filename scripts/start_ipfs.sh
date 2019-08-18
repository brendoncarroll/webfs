#!/bin/sh

docker run -it --rm --name ipfs-node \
  -p 8080:8080 -p 4001:4001 -p 127.0.0.1:5001:5001 \
  jbenet/go-ipfs:latest

