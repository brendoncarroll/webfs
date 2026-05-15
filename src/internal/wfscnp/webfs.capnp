@0x8bfbc7e27592cd31;

using Go = import "/go.capnp";

$Go.package("wfscnp");
$Go.import("github.com/brendoncarroll/webfs/src/internal/wfscnp");

enum TypeCode {
  unknown @0;
  regularFile @1;
  dir @2;
  volumeLink @3;
}

# TAI64N is a TAI64 timestamp with nanosecond resolution
struct TAI64N {
    seconds @0 :UInt64;
    nanoseconds @1 :UInt32;
}

struct Node {
  createdAt @0: TAI64N;
  modifiedAt @1 :TAI64N;
  refCount @2 :UInt32;
  rev @3 :UInt64;
  mode @7: UInt32;

  payload :union {
    file @4 :File;
    dir @5 :Dir;
    volumeLink @6: VolumeLink;
  }
}

struct Dir {
}

struct File {
  size @0 :UInt64;
  blockSize @1 :UInt32;
}

struct VolumeLink {
    nodeID @0 :Data;
    oid @1 :Data;
}

struct Session {
    createAt @0: TAI64N;
    # ttl is the time to live in seconds.
    ttl @1: UInt32;
    touchedAt @2: TAI64N;
    lockCount @3: UInt32;
}

struct LockState {
    # holder is the inet256.ID that owns this lock.
    holder @0: Data;
    # kind is a lock-mode enum value defined in webfs.
    kind @1: UInt16;
    # start is the first byte in the locked range.
    start @2: UInt64;
    # length is the number of bytes in the locked range. 0 means to EOF.
    length @3: UInt64;
}

struct Ref {
    cid @0: UInt256;
    dek @1: UInt256;
}

# UInt256 is suitable for storing blobcache.CIDs
struct UInt256 {
    # a is the lowest bits, little endian
    a @0: UInt64;
    b @1: UInt64;
    c @2: UInt64;
    # d is the highest bits, little endian
    d @3: UInt64;
}
