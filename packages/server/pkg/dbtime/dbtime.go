//nolint:revive // exported
package dbtime

import "time"

type DBTimeData time.Time

func (t DBTimeData) Time() time.Time {
	return DBTime(time.Time(t))
}

func DBNow() time.Time {
	return DBTime(time.Now())
}

func DBTime(t time.Time) time.Time {
	return t.UTC()
}
