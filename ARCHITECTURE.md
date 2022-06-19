# WebFS Architecture

WebFS depends on two interfaces: `Stores` and `Cells`.

### Stores
Stores are content-addressed stores.
They support two fundamental operations

```
Post(data) -> hash
Get(hash) -> data
```
The standard interface is defined by the [`cadata`](http://github.com/brendoncarroll/go-state/tree/master/cadata) package from the go-state library.

Store implementations can be found in `pkg/stores/`

### Cells
Cells in WebFS are like cells in a spreadsheet, a holder of a data which can change over time.
The compare-and-swap operation (`CAS(current, next)`) allows writes which will be synchronized with other WebFS instances writing to the same cell.

Cells provide two fundamental operations:
```
Get() -> current
CAS(prev, next []byte) -> current
```

The standard interface is defined by the [`cells`](https://github.com/brendoncarroll/go-state/tree/master/cells) package from the go-state library.

Cell implementations can be found in `pkg/cells/`

### Formats
WebFS uses [GotFS](https://github.com/gotvc/got/tree/master/pkg/gotfs) from the [Got](https://github.com/gotvc/got) version control system for storing file data.
