package webref

import (
	"context"
	"errors"
	fmt "fmt"

	"github.com/brendoncarroll/webfs/pkg/stores"
)

type Getter interface {
	Get(ctx context.Context, ref *Ref) ([]byte, error)
}

type Poster interface {
	Post(ctx context.Context, data []byte) (*Ref, error)
	MaxBlobSize() int
}

type Deleter interface {
	Delete(ctx context.Context, ref *Ref) error
}

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
		for _, ref2 := range r.Mirror.Refs {
			us := GetURLs(ref2)
			ret = append(ret, us...)
		}
		return ret
	case *Ref_Annotated:
		return GetURLs(r.Annotated.Ref)
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

func Check(ctx context.Context, s stores.Check, ref *Ref) []RefStatus {
	switch r := ref.Ref.(type) {
	case *Ref_Url:
		err := s.Check(ctx, r.Url)
		return []RefStatus{
			{URL: r.Url, Error: err},
		}
	case *Ref_Mirror:
		return CheckMirror(ctx, s, r.Mirror)
	case *Ref_Crypto:
		return Check(ctx, s, r.Crypto.Ref)
	case *Ref_Annotated:
		return Check(ctx, s, r.Annotated.Ref)
	default:
		panic("invalid ref type")
	}
}
