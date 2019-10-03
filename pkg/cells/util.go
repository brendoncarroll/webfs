package cells

import (
	"context"
	"errors"
)

func ForcePut(ctx context.Context, c Cell, data []byte, retries int) error {
	for i := 0; i < retries; i++ {
		cur, err := c.Get(ctx)
		if err != nil {
			return err
		}
		success, err := c.CAS(ctx, cur, data)
		if err != nil {
			return err
		}
		if success {
			return nil
		}
	}
	return errors.New("out of retries")
}
