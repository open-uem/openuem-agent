package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterWinGetLog(t *testing.T) {
	log := "2026-11-16 15:08:40.123 [CORE] WinGet, version [1.9.25200], activity [{1234}]\r\n" +
		"2026-11-16 15:08:40.200 [REPO] Adding source: Name[OpenUEM], Type[Microsoft.Rest]\r\n" +
		"2026-11-16 15:08:41.000 [FAIL] HttpClientHelper.cpp(57)\\WindowsPackageManager.dll: Exception(1) tid(1a2b) 80072F8F A security error occurred\r\n" +
		"2026-11-16 15:08:41.001 [CLI ] Caught exception: A security error occurred\r\n" +
		"2026-11-16 15:08:41.002 [CLI ] Terminating context: 0x8a150045 at SourceFlow.cpp:170\r\n" +
		"2026-11-16 15:08:41.003 [CORE] Leaving\r\n"

	assert.Equal(t, []string{
		"2026-11-16 15:08:41.000 [FAIL] HttpClientHelper.cpp(57)\\WindowsPackageManager.dll: Exception(1) tid(1a2b) 80072F8F A security error occurred",
		"2026-11-16 15:08:41.001 [CLI ] Caught exception: A security error occurred",
		"2026-11-16 15:08:41.002 [CLI ] Terminating context: 0x8a150045 at SourceFlow.cpp:170",
	}, FilterWinGetLog(log, 15))

	assert.Len(t, FilterWinGetLog(log, 2), 2, "only the last lines")
	assert.Equal(t, "2026-11-16 15:08:41.002 [CLI ] Terminating context: 0x8a150045 at SourceFlow.cpp:170", FilterWinGetLog(log, 1)[0])
	assert.Empty(t, FilterWinGetLog("", 15))
}
