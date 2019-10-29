package ccutil

import "testing"

func TestCountUp(t *testing.T) {
	const N = 3
	cu := NewCountUp()

	for i := 0; i < N; i++ {
		cu.Wait(i)
		cu.Done(i)
	}
}
