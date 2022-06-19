# WebFS

WebFS is a filesystem built on top of the web.

WebFS started after looking for a way to use IPFS as a Dropbox replacement,
and not finding any really solid solutions.  I also wanted to be able to fluidly move
my data between traditional storage providers like Dropbox, MEGA, or Google Drive while testing the waters of new p2p storage systems, like Swarm or Filecoin.

If you have ever thought "x can probably be used as a file system", but didn't want to actually write the file system part, WebFS might be of benefit to you.
You can probably turn x into a file system with WebFS by writing a new `Store` or `Cell` implementation.

## Quick Links
[CLI Docs](./doc/10_CLI.md)

[Volume Specs Docs](./doc/11_Volume_Specs.md)

[ARCHITECTURE.md](./ARCHITECTURE.md)

## Installation
Installs to `$GOPATH/bin` with `make install`

## Getting Started
A simple volume spec using the filesystem for storage

```json
{
    "cell": {
        "file": "CELL_DATA",
    },
    "store": {
        "fs": "BLOBS",
    }
}
```
This configuration will create and write to a file `./CELL_DATA` and a directory `./BLOBS`, so plan accordingly.

To serve the files over http
```shell
$ webfs http --root myvolume.webfs
  serving at http://127.0.0.1:7007
```

## Examples
There are examples in the `/examples` directory.
The examples assume you have the `webfs` executable on your `$PATH`.

You can also use
```go run ../../cmd/webfs``` instead of ```webfs``` if you don't want to set that up.

## Community
Questions and Discussion happening on Matrix.

![Matrix](https://img.shields.io/matrix/webfs:matrix.org?label=%23webfs%3Amatrix.org&logo=matrix)
