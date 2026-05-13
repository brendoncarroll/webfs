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
