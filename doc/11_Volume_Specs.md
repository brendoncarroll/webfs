# Volume Specs
WebFS volumes are specified by JSON configuration files in the filesystem with names ending in `.webfs`

WebFS volumes consist of a cell, a store, and a salt.
The salt is only used when adding files.

Every volume spec should look something like this

```json
{
    "cell": {
        ...
    },
    "store" : {
        ...
    },
    "salt": "hJYTuuOky0Q4w25olAF+UY894bnNgRkXO2OIyeRd+yE="
}
```

# Cells

## `file`
e.g.
```json
{
   "cell": {
        "file": "path/to/cell"
    }
    ...
}
```

## `http`
e.g.
```json
{
   "cell": {
        "http": {
            "url": "http://example.com/cells/1234",
            "headers": {
                "X-My-Headers": "header-value",
            }
        }
    }
    ...
}
```

## `aead`
e.g.
```json
{
   "cell": {
        "aead": {
            "inner": {
                ...
            },
            "algo": "chacha20poly1305",
            "secret": "hJYTuuOky0Q4w25olAF+UY894bnNgRkXO2OIyeRd+yE="
        }
    }
    ...
}
```

## `got_branch`
e.g.
```json
{
   "cell": {
        "got_branch": {
            "inner": {
                ...
            }, 
        }
    }
    ...
}
```

# Stores

## `fs`
e.g.
```json
{
    "store": {
        "fs": "path/to/dir",
    }
    ...
}
```

## `http`
e.g.
```json
{
    "store": {
        "http": "path/to/dir",
    }
    ...
}
```

## `blobcache`
e.g.
```json
{
    "store": {
        "blobcache": {}
    }
    ...
}
```

## `ipfs`
e.g.
```json
{
    "store": {
        "ipfs": {},
    }
    ...
}
```
