package webref

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscatedLen(t *testing.T) {
	testCases := []struct {
		x, y int
	}{
		{x: 10, y: 16},
		{x: 0, y: 0},
		{x: 1, y: 1},
		{x: 2, y: 2},
		{x: 200, y: 256},
		{x: 32, y: 32},
	}

	for _, tc := range testCases {
		actual := obfuscatedLength(tc.x)
		assert.Equal(t, tc.y, actual)
	}
}
