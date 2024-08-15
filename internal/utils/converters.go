package utils

import (
	"fmt"
	"strings"
)

func ConvertInt32ArrayToString(a []int32) string {
	str := ""
	for _, code := range a {
		str += string(rune(code))
	}
	return str
}

func ConvertBytesToUnits(size uint64) string {
	units := fmt.Sprintf("%d MB", size/1_000_000)
	if size/1_000_000 >= 1000 {
		units = fmt.Sprintf("%d GB", size/1_000_000_000)
	}
	if size/1_000_000_000 >= 1000 {
		units = fmt.Sprintf("%d TB", size/1_000_000_000_000)
	}
	if size/1_000_000_000_000 >= 1000 {
		units = fmt.Sprintf("%d PB", size/1_000_000_000_000)
	}
	return strings.TrimSpace(units)
}
