package deploy

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-uem/nats"
	"github.com/stretchr/testify/assert"
)

func TestParseWinGetUpgradeOutput(t *testing.T) {
	widths := []int{34, 30, 15, 15}
	output := "   - \r   \\ \r   | \r" +
		"  ██████████████████████████████  1024 KB / 1024 KB\r\n" +
		row(widths, "Name", "Id", "Version", "Available", "Source") + "\r\n" +
		"-----------------------------------------------------------------------------------------------\r\n" +
		row(widths, "Mozilla Firefox (x64 es-ES)", "Mozilla.Firefox.es-ES", "131.0", "132.0.1", "winget") + "\r\n" +
		row(widths, "7-Zip 23.01 (x64)", "7zip.7zip", "23.01", "24.08", "winget") + "\r\n" +
		row(widths, "Microsoft Visual C++ 2015-2022 R…", "Microsoft.VCRedist.2015+.x64", "14.38.33135.0", "14.40.33810.0", "winget") + "\r\n" +
		row(widths, "Some Very Long Application Name", "Vendor.SomeVeryLongApplicat…", "1.0", "2.0", "winget") + "\r\n" +
		row(widths, "Old Tool", "Vendor.OldTool", "< 2.0", "2.1", "winget") + "\r\n" +
		"5 upgrades available.\r\n" +
		"\r\n" +
		"The following packages have an upgrade available, but require explicit targeting for upgrade:\r\n" +
		row(widths, "Name", "Id", "Version", "Available", "Source") + "\r\n" +
		"-----------------------------------------------------------------------------------------------\r\n" +
		row(widths, "Pinned", "Vendor.Pin", "1.0", "2.0", "winget") + "\r\n"

	// The truncated id is not reported and the pinned table is ignored
	assert.Equal(t, []nats.AvailableUpdate{
		{PackageName: "Mozilla Firefox (x64 es-ES)", PackageId: "Mozilla.Firefox.es-ES", InstalledVersion: "131.0", AvailableVersion: "132.0.1", Source: "winget"},
		{PackageName: "7-Zip 23.01 (x64)", PackageId: "7zip.7zip", InstalledVersion: "23.01", AvailableVersion: "24.08", Source: "winget"},
		{PackageName: "Microsoft Visual C++ 2015-2022 R", PackageId: "Microsoft.VCRedist.2015+.x64", InstalledVersion: "14.38.33135.0", AvailableVersion: "14.40.33810.0", Source: "winget"},
		{PackageName: "Old Tool", PackageId: "Vendor.OldTool", InstalledVersion: "< 2.0", AvailableVersion: "2.1", Source: "winget"},
	}, ParseWinGetUpgradeOutput(output))
}

func TestParseWinGetUpgradeOutputLocalized(t *testing.T) {
	// German headers and a French footer that has as many words as a row
	widths := []int{11, 16, 8, 10}
	output := row(widths, "Name", "ID", "Version", "Verfügbar", "Quelle") + "\n" +
		"----------------------------------------------------\n" +
		row(widths, "Notepad++", "Notepad++.Npp", "8.6", "8.7", "winget") + "\n" +
		"2 mises à niveau disponibles.\n"

	updates := ParseWinGetUpgradeOutput(output)
	assert.Len(t, updates, 1)
	assert.Equal(t, "Notepad++.Npp", updates[0].PackageId)
	assert.Equal(t, "8.7", updates[0].AvailableVersion)
}

func TestParseWinGetUpgradeOutputWideCharacters(t *testing.T) {
	// winget pads using the display width, wide characters take two columns
	// "微信" is four columns wide, so it is followed by 9 spaces instead of 11
	output := "Name         Id              Version Available\n" +
		"----------------------------------------------\n" +
		"微信         Tencent.WeChat  3.9     4.0\n" +
		"WeChat Beta  Tencent.Beta    3.9     4.0\n"

	updates := ParseWinGetUpgradeOutput(output)
	assert.Equal(t, []nats.AvailableUpdate{
		{PackageName: "微信", PackageId: "Tencent.WeChat", InstalledVersion: "3.9", AvailableVersion: "4.0"},
		{PackageName: "WeChat Beta", PackageId: "Tencent.Beta", InstalledVersion: "3.9", AvailableVersion: "4.0"},
	}, updates)
}

func TestParseWinGetUpgradeOutputNoUpdates(t *testing.T) {
	assert.Empty(t, ParseWinGetUpgradeOutput("No installed package found matching input criteria.\r\n"))
	assert.Empty(t, ParseWinGetUpgradeOutput(""))
}

// row pads values to the given column widths like winget does
func row(widths []int, values ...string) string {
	line := ""
	for i, v := range values {
		if i < len(widths) {
			v += strings.Repeat(" ", max(1, widths[i]-utf8.RuneCountInString(v)))
		}
		line += v
	}
	return line
}

func TestIsValidPackageID(t *testing.T) {
	for _, id := range []string{"Mozilla.Firefox", "org.mozilla.firefox", "Microsoft.VCRedist.2015+.x64", "9NBLGGH4NNS1", "google-chrome"} {
		assert.True(t, IsValidPackageID(id), id)
	}
	for _, id := range []string{"", "--force", "a b", "x;calc", "x'y", "a`b", "a$(b)"} {
		assert.False(t, IsValidPackageID(id), id)
	}
}
