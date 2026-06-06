package zip

import "time"

func dosDateTime(t time.Time) (uint16, uint16) {
	t = t.Local()

	dosTime :=
		uint16(t.Second()/2) |
			uint16(t.Minute())<<5 |
			uint16(t.Hour())<<11

	dosDate :=
		uint16(t.Day()) |
			uint16(t.Month())<<5 |
			uint16(t.Year()-1980)<<9

	return dosTime, dosDate
}
