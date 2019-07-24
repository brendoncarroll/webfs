package wrds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexOf(t *testing.T) {
	tree := Tree{
		Entries: []TreeEntry{
			{Key: []byte("bbb")},
			{Key: []byte("ccc")},
			{Key: []byte("ddd")},
		},
	}
	var i int
	i = tree.indexOf([]byte("aaa"))
	assert.Equal(t, -1, i)
	i = tree.indexOf([]byte("eee"))
	assert.Equal(t, 2, i)
}
