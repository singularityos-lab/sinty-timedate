package timedate

import (
	"fmt"
	"time"

	"github.com/singularityos-lab/libsingularity/src/system/core/go/sintysystem"
)

func ReadRTC() (time.Time, error) {
	t, err := sintysystem.RTC()
	if err != nil {
		return time.Time{}, fmt.Errorf("read RTC: %w", err)
	}
	return time.Date(int(t.Year), time.Month(t.Month), int(t.Day), int(t.Hour), int(t.Minute), int(t.Second), 0, time.UTC), nil
}

func WriteRTC(wall time.Time) error {
	err := sintysystem.SetRTC(sintysystem.RTCTime{
		Year: int32(wall.Year()), Month: int32(wall.Month()), Day: int32(wall.Day()),
		Hour: int32(wall.Hour()), Minute: int32(wall.Minute()), Second: int32(wall.Second()),
	})
	if err != nil {
		return fmt.Errorf("write RTC: %w", err)
	}
	return nil
}
