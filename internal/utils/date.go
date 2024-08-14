package utils

import "time"

// Credits: https://stackoverflow.com/questions/21053427/check-if-two-time-objects-are-on-the-same-date-in-go
func DateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
