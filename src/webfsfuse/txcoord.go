package webfsfuse

import (
	"context"

	"github.com/brendoncarroll/webfs/src/webfs"
)

// txCoord is a transaction coordinator
type txCoord struct {
	webfs   *webfs.System
	rootCfg webfs.VolumeConfig
}

func (tc *txCoord) view(ctx context.Context, fn func(*webfs.FSTx) error) error {
	tx, err := tc.webfs.View(ctx, tc.rootCfg)
	if err != nil {
		return err
	}
	defer tx.Abort(ctx)
	return fn(&tx.FSTx)
}

func (tc *txCoord) modify(ctx context.Context, fn func(*webfs.FSTx) error) error {
	tx, err := tc.webfs.Modify(ctx, tc.rootCfg)
	if err != nil {
		return err
	}
	defer tx.Abort(ctx)
	if err := fn(&tx.FSTx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (tc *txCoord) sync(ctx context.Context) error {
	// TODO: this is currently a no-op since we create a tx per view or modify call.
	return nil
}
