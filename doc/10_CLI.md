# WebFS Command Line Interface

When performing an operation on a WebFS filesystem, the root config must always be provided.
On the command line, this is done using the `--root` or `-r` flag.

In practice this usually lookes something like this:
```shell
$ webfs -r ./path/to/webfs_root.json ls
```

# Primitive Operations

## `webfs add <dst> <src>`
Adds files to WebFS.
- `dst` is a path within WebFS.
- `src` is assumed to be a file in the local filesystem.
URLs may also eventually be supported.

## `webfs ls <path>`
Like UNIX's `ls`, but within the WebFS filesystem.
Lists the paths which are children of `path`

## `webfs rm <path>`
Removes a file or directory.
This is like `rm -rf`.

## `webfs edit <path>`
Edit a file in WebFS using `$EDITOR`.
Defaults to `vim` if `$EDITOR` is not set.
This is useful for configuring a subvolume in a `*.webfs` file within the filesystem.

## `webfs mkdir <path>`
Creates all directories along path.
Similar to `mkdir -p <path>`.

# Servers
## `webfs http [--addr]`
Serves files over HTTP.

## `webfs nfs [--addr]`
Serves files ovver NFS.

## `webfs mount [--path]`
Mounts a fuse filesystem at path.
