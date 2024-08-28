package dbtime

import "time"

func DBNow() time.Time {
	return DBTime(time.Now())
}

func DBTime(t time.Time) time.Time {
	return t.UTC()
}
