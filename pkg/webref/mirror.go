package webref

import (
	"context"
	fmt "fmt"
	"log"
	mrand "math/rand"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func GetMirror(ctx context.Context, s stores.Read, m *Mirror) ([]byte, error) {
	l := len(m.Refs)
	count := 0
	i := mrand.Int() % l
	errs := []error{}
	for count < l {
		ref := m.Refs[i]
		data, err := Get(ctx, s, *ref)
		if err == nil {
			return data, nil
		}
		errs = append(errs, err)
		log.Println(err)

		i = (i + 1) % l
		count++
	}
	return nil, fmt.Errorf("Errors from all replicas")
}

func DeleteMirror(ctx context.Context, s stores.Delete, r *Mirror) error {
	for _, x := range r.Refs {
		if err := Delete(ctx, s, *x); err != nil {
			return err
		}
	}
	return nil
}

func CheckMirror(ctx context.Context, s stores.Check, r *Mirror) []RefStatus {
	stats := []RefStatus{}
	for _, x := range r.Refs {
		stats2 := Check(ctx, s, x)
		stats = append(stats, stats2...)
	}
	return stats
}
