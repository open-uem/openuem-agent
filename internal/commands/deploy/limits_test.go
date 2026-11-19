package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLocalDrivePath(t *testing.T) {
	assert.True(t, isLocalDrivePath(`C:\Users\user\AppData\Local`))
	assert.True(t, isLocalDrivePath(`d:\profiles\user\AppData\Local`))
	for _, p := range []string{`\\attacker\share`, `\\?\UNC\attacker\share`, `//attacker/share`, `C:relative`, `AppData\Local`, ``, `\\.\pipe\x`} {
		assert.False(t, isLocalDrivePath(p), p)
	}
}
