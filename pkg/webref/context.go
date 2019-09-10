package webref

import "context"

type ctxKey int

const ctxKeyCodec = iota

func SetCodecCtx(ctx context.Context, codec string) context.Context {
	return context.WithValue(ctx, ctxKeyCodec, codec)
}

func GetCodecCtx(ctx context.Context) string {
	v := ctx.Value(ctxKeyCodec)
	x, _ := v.(string)
	if x == "" {
		x = CodecJSON
	}
	return x
}
