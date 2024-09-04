package helpers

import "time"

// AddDays adds x days to the current date and returns the new date
func AddDays(x int, date time.Time) time.Time {
	duration := time.Duration(x) * 24 * time.Hour
	newDate := date.Add(duration)

	return newDate
}
