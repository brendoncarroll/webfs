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
`superblock.webfs` was edited to require 1 replica with the "ipfs://" prefix
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
ipfs://QmPmdC2fDDvj4QJ6kKL2JRD8QNi1hYDgseK6D61N1NpmoD
ipfs://QmafqGhTTk2FvXG77A3S8oBUZx784hq1LCAcfq3Pm1Pjhq
ipfs://QmUGE3tLnh6HDSHuVhQg9H5ECyURLjWiuwDUe3vkLiEzTE
ipfs://QmcCLcrLUCchMqWhuGb7WFaZXUFieJwGtotL1YzkyDVxnM
```
