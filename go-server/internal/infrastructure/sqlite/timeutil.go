package sqlite

import "time"

// sqliteTimeLayout mirrors the shape of SQLite's CURRENT_TIMESTAMP: a naive
// UTC wall clock with optional fractional seconds.
const sqliteTimeLayout = "2006-01-02 15:04:05.999999"

// utcTimestamp renders t in UTC using the same shape as CURRENT_TIMESTAMP so
// every timestamp column stores the same clock and text comparisons (ordering,
// retention windows) stay meaningful.
func utcTimestamp(t time.Time) string {
	return t.UTC().Format(sqliteTimeLayout)
}

// utcTimestampArg is utcTimestamp for nullable columns; nil is kept as SQL NULL.
func utcTimestampArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return utcTimestamp(*t)
}
