# WebFS

WebFS is a filesystem built on top of the web.

This project started after looking for a way to use IPFS as a Dropbox replacement,
and not finding any really solid solutions.  I also wanted to be able to fluidly move
my data between paid storage providers like Dropbox, MEGA, or Google Drive while testing the waters of exciting new p2p storage systems.

The design is broken up into layers.
Although it is possible for all these components to be arbitrarily stacked, there turns out to
be very good reasons to stack them in a certain way, hence the numbering.

## L0 - Content Adressed Blobs
This is the interface WebFS expects underneath it.  This is where the storage economy exists, it is comprised of:
- Self interested nodes part of Filecoin, or Swarm.
- Altrusitic nodes in Bittorrent, IPFS, or Freenet
- Paid Storage e.g. MEGA, Dropbox, etc.
- Social Favors e.g. a friend hosts SFTP on an old box; your public key is allowed.

These entities should really task themselves with one thing. Serve up content addressed blobs quickly.  You give them a hash, they give you a blob.  Maybe you settle up cash after/before.

References to data are stored as URLs.  It is up to the schema handler for the URL to determine how the content addressing works.  With IPFS it's multihash, with Dropbox, or FTP it could just be naming the files as the SHA3 and checking you got what you asked for.

## L1 - Crypto
This is the first layer of WebFS. Storage providers should not be able to scan through blobs and learn anything useful.
The structure of the blobs and their relation to other blobs should only be known to the requester, not the provider.

Public data is encrypted convergently, everything else is encrypted with a random key.  The random key and the encryption algorithm are included in a "L1 Ref". The next layer deals in `l1.Ref`s instead of URLs.

## L2 - Replication / Packing
This layer deals with mirroring, erasure coding, and packing. Anything that can take one blob and make many, or cram many into one.
This layer will probably be passthrough for many usecases, some might use mirroring.
Compression through packing, and packing for RAID5 or Reed Solomon are lower priority right now.

## L3 - Merkle Data Structures
In order to index many blobs we use a btree similar to real filesystems operating on block devices.
This allows us to have large directories and most importantly a file abstraction, so you are not constrained to the maximum blob size of the storage layer.

## L4 - WebFS Objects
WebFS takes a typical copy-on-write merkle tree approach similar to git or IPFS.  There are 3 objects that make up the data model.
- Files.  Just a btree from file offsets to data.
- Directories. A btree from strings to other WebFS objects.
- Cells. An authority on part of the file system. The contents of cells are mutable and they can contain any other WebFS object.  Cells will be invisible to a client of the file system.

WebFS gets everything it needs from a config file called the "superblock".
The superblock contains a representation of the `RootCell`, which contains a reference to a directory or another cell.
There are many possible implementations of a cell, so the cell inside the root cell could have a networked implementation and everything below it is synced with a server, or a p2p consensus mechanism.

Without any other cells all writes would propagate to the root.
This is like git; it gives a globally consistent view of the entire tree.