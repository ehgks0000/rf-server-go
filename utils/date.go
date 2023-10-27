package utils

import "time"

func getPastDateISOString(days int, now time.Time) string {
	pastDate := now.AddDate(0, 0, -days)
	isoString := pastDate.Format(time.RFC3339)
	return isoString // removing the seconds and timezone offset to match the TypeScript function
	// return isoString[:len(isoString)-5] // removing the seconds and timezone offset to match the TypeScript function
}

type DateRange struct {
	Gte string
	Lte string
}

// GetDateRange returns a map with the ISO strings of 'now' and 30 days before 'now'.
func GetDateRange(now time.Time) DateRange {
	thirtyDaysAgoISOString := getPastDateISOString(30, now)
	nowISOString := now.Format(time.RFC3339) // removing the seconds and timezone offset to match the TypeScript function

	return DateRange{
		Gte: thirtyDaysAgoISOString,
		Lte: nowISOString,
	}
}
