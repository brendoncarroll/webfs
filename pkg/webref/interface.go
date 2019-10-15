package webref

import (
	"context"
	"errors"
	fmt "fmt"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

func errRefType(ref isRef_Ref) error {
	return fmt.Errorf("invalid ref type: %T", ref)
}

func Get(ctx context.Context, s stores.Read, ref Ref) ([]byte, error) {
	switch r := ref.Ref.(type) {
	case *Ref_Url:
		return GetSingle(ctx, s, r.Url)
	case *Ref_Crypto:
		return GetCrypto(ctx, s, r.Crypto)
	case *Ref_Mirror:
		return GetMirror(ctx, s, r.Mirror)
	case *Ref_Annotated:
		return Get(ctx, s, *r.Annotated.Ref)
	default:
		return nil, errRefType(r)
	}
}

func Post(ctx context.Context, s stores.Post, opts Options, data []byte) (*Ref, error) {
	if len(opts.Replicas) == 0 {
		return nil, errors.New("no replicas to post to")
	}
	if len(opts.Replicas) > 1 {
		return PostMirror(ctx, s, opts, data)
	}

	var prefix string
	for k, v := range opts.Replicas {
		if v > 1 {
			return PostMirror(ctx, s, opts, data)
		}
		prefix = k
		break
	}

	return PostCrypto(ctx, s, opts, prefix, data)
}

func Delete(ctx context.Context, s stores.Delete, ref Ref) error {
	switch r := ref.Ref.(type) {
	case *Ref_Url:
		return DeleteSingle(ctx, s, r.Url)
	case *Ref_Crypto:
		return DeleteCrypto(ctx, s, r.Crypto)
	case *Ref_Mirror:
		return DeleteMirror(ctx, s, r.Mirror)
	default:
		return errors.New("invalid ref type")
	}
}

func GetURLs(ref *Ref) []string {
	switch r := ref.Ref.(type) {
	case *Ref_Url:
		return []string{r.Url}
	case *Ref_Crypto:
		return GetURLs(r.Crypto.Ref)
	case *Ref_Mirror:
		ret := []string{}
		for _, ref2 := range r.Mirror.Replicas {
			us := GetURLs(ref2)
			ret = append(ret, us...)
		}
		return ret
	default:
		panic("invalid ref type")
	}
}

type RefStatus struct {
	URL   string
	Error error
}

func (rs RefStatus) IsAlive() bool {
	return rs.Error == nil
}

func Check(ctx context.Context, s stores.Check, ref Ref) []RefStatus {
	switch r := ref.Ref.(type) {
	case *Ref_Url:
		return CheckSingle(ctx, s, r.Url)
	case *Ref_Crypto:
		return CheckCrypto(ctx, s, r.Crypto)
	default:
		panic("invalid ref type")
	}
}
