## Setup for the curious
This is what was run to create this example.
It requires having a running IPFS daemon.
There is a script to spin one up in docker with no persistent storage in `scripts`

```
$ webfs new fs
$ webfs mkdir /
$ webfs mkdir mydir
$ webfs import ./hello.txt hello.txt
```
`superblock.json` was edited to require 1 replica with the "ipfs://" prefix
```
{
  "options": {
    "dataOpts": {
      ...
      "replicas": {
        "ipfs://": 1
      },
    },
    ...
  }
}
```

## Demo Starts Here
List the contents of the root directory.
```
$ webfs ls
/
 hello.txt                    12B Object{File}                  
 mydir                         0B Object{Dir}       
```

List the contents of a subdirectory
```
$ webfs ls mydir
mydir
 hello_again.txt              12B Object{File}
```

Cat out a file
```
$ webfs cat hello.txt
hello world
```

Get the URLs for all the blobs in the filesystem
```
$ webfs urldump
ipfs://QmedPkM6PsjtVCNwag7zmeqRomhau3KCHiW27gxRcaNLWL
ipfs://QmVkCgNh3ELa1mGfyzZ2vTUwNa2Ahndckb9xDmFbrHYpqD
ipfs://QmdzinBDW1KSp2HcwKLtk8HfhvXbpvcGBi8dYQZ8GgbNoK
ipfs://QmXpD7aZEUPG61AN9WxZX7pRKsQ9SPVJEVeQ3jUUfGQywp
ipfs://QmaMN4MFBFsAeZ5monK2m3dpSPb8M7xVYZagHtNG4jAVGb
ipfs://QmXpD7aZEUPG61AN9WxZX7pRKsQ9SPVJEVeQ3jUUfGQywp
```
