package runtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCappedBuffer(t *testing.T) {
	b := &CappedBuffer{Max: 10}
	n, err := b.Write([]byte("0123456789abcdef"))
	assert.NoError(t, err, "the writer keeps accepting, so the command isn't blocked")
	assert.Equal(t, 16, n)
	b.Write([]byte("more"))
	assert.True(t, strings.HasPrefix(b.String(), "0123456789"))
	assert.Contains(t, b.String(), "output truncated")
	assert.NotContains(t, b.String(), "abc")
}
