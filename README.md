# WebFS

WebFS is a filesystem built on top of the Blobcache.

WebFS started after looking for a way to use IPFS as a Dropbox replacement,
and not finding any really solid solutions.  I also wanted to be able to fluidly move
my data between traditional storage providers like Dropbox, MEGA, or Google Drive while testing the waters of new p2p storage systems, like Swarm or Filecoin.

Blobache provides a uniform API that makes everything look like a content-addressed store, supporting transactional modification.
WebFS chains together these Volumes into Drives, which represent consistent subregions of the filesystem.
Volumes can be locally persisted, persisted on another Blobcache node, or on external systems like Git, commodity object storage (S3, GCS, etc.), or a blockchain.

If you have ever thought "*x* can probably be used as a file system", but didn't want to actually write the file system part, WebFS might be of benefit to you.
You can probably turn *x* into a file system with WebFS by implementing a new volume backend for Blobcache.
