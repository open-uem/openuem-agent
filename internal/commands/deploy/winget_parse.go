package deploy

import (
	"strings"
	"unicode"

	"github.com/open-uem/nats"
)

// ParseWinGetUpgradeOutput parses the table printed by `winget upgrade` when it is
// run without a package query. Column headers are localized, so columns are located
// by their position in the header line, not by their names. Rows whose package id
// has been truncated by winget (ending in "…") are skipped as they can't be targeted.
func ParseWinGetUpgradeOutput(output string) []nats.AvailableUpdate {
	lines := cleanWinGetOutput(output)

	// The header is the line right above a line made only of dashes
	header := -1
	for i := 0; i < len(lines)-1; i++ {
		separator := strings.TrimSpace(lines[i+1])
		if len(separator) > 10 && strings.Trim(separator, "-") == "" && strings.TrimSpace(lines[i]) != "" {
			header = i
			break
		}
	}
	if header == -1 {
		return nil
	}

	columns := columnStarts(lines[header])
	// Name, Id, Version, Available and an optional Source column
	if len(columns) < 4 || len(columns) > 5 {
		return nil
	}

	updates := []nats.AvailableUpdate{}
	for _, line := range lines[header+2:] {
		fields, ok := splitColumns(line, columns)
		if !ok {
			// The table ends at the first line that is not a row, e.g. the
			// "N upgrades available." footer or an empty line
			break
		}

		id := fields[1]
		if strings.HasSuffix(id, "…") || !IsValidPackageID(id) {
			continue
		}

		u := nats.AvailableUpdate{
			PackageName:      strings.TrimSuffix(fields[0], "…"),
			PackageId:        id,
			InstalledVersion: fields[2],
			AvailableVersion: fields[3],
		}
		if len(fields) == 5 {
			u.Source = fields[4]
		}
		updates = append(updates, u)
	}

	return updates
}

// cleanWinGetOutput splits winget output in lines, removing the progress
// spinner and bars that winget writes using carriage returns
func cleanWinGetOutput(output string) []string {
	lines := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if i := strings.LastIndex(line, "\r"); i >= 0 {
			line = line[i+1:]
		}
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.ContainsAny(line, "█▒") {
			continue
		}
		if t := strings.TrimSpace(line); t == "-" || t == "\\" || t == "|" || t == "/" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// columnStarts returns the display column where each header word starts
func columnStarts(header string) []int {
	starts := []int{}
	col := 0
	inWord := false
	for _, r := range header {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			starts = append(starts, col)
		}
		col += runeWidth(r)
	}
	return starts
}

// splitColumns cuts a row at the given display columns. A row is only valid if
// every column but the first starts right after a space and has a value
func splitColumns(line string, starts []int) ([]string, bool) {
	fields := make([]strings.Builder, len(starts))
	current := 0
	col := 0
	var previous rune
	for _, r := range line {
		for current+1 < len(starts) && col >= starts[current+1] {
			current++
			// winget pads columns with spaces, so a column can't start mid-word
			if col == starts[current] && previous != ' ' {
				return nil, false
			}
		}
		fields[current].WriteRune(r)
		col += runeWidth(r)
		previous = r
	}

	values := make([]string, len(starts))
	for i := range fields {
		values[i] = strings.TrimSpace(fields[i].String())
		// Id, Version and Available are mandatory
		if i <= 3 && values[i] == "" {
			return nil, false
		}
		// Ids and sources never contain spaces
		if (i == 1 || i == 4) && strings.ContainsRune(values[i], ' ') {
			return nil, false
		}
	}
	return values, true
}

// runeWidth returns the number of console columns used by a rune. East Asian
// wide characters take two columns, which winget accounts for when padding.
func runeWidth(r rune) int {
	switch {
	case r == 0 || unicode.Is(unicode.Mn, r):
		return 0
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0xA4CF && r != 0x303F,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1F64F,
		r >= 0x1F900 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	default:
		return 1
	}
}

// IsValidPackageID checks that a package id can be safely passed as an argument to
// a package manager: it must not be empty, look like a flag or contain whitespace
func IsValidPackageID(id string) bool {
	if id == "" || strings.HasPrefix(id, "-") || len(id) > 256 {
		return false
	}
	return !strings.ContainsFunc(id, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("\"'`&|;<>$", r)
	})
}
