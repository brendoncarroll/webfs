package webfs

import (
	"context"
	"log"

	"github.com/brendoncarroll/webfs/pkg/webref"
)

type RefStatus struct {
	webref.RefStatus
	PartOf string
}

func (rs RefStatus) IsAlive() bool {
	return rs.RefStatus.IsAlive()
}

func (wfs *WebFS) Check(ctx context.Context, ch chan RefStatus) error {
	err := wfs.ParDo(ctx, func(o Object) bool {
		s := o.getStore()

		cont, err := o.RefIter(ctx, func(ref Ref) bool {
			rs, err := webref.Check(ctx, s, ref)
			if err != nil {
				log.Println(err)
				return false
			}

			for i := range rs {
				ch <- RefStatus{
					RefStatus: rs[i],
					PartOf:    o.String(),
				}
			}
			return true
		})
		if err != nil {
			log.Println(err)
		}
		return cont
	})
	close(ch)

	return err
}
