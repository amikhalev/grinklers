package sched

import (
	"testing"
	"time"
)

func TestTimeOfDay(t *testing.T) {
	tod := TimeOfDay{1, 1, 1, 0}
	dur := tod.Duration()
	if dur.Seconds() != 1+60+3600 {
		t.Error("TimeOfDay#Duration")
	}
	if tod.String() != "01:01:01:0000" {
		t.Errorf("TimeOfDay#String() = %s", tod.String())
	}
}

func TestNextDay(t *testing.T) {
	test := func(day int, weekday time.Weekday, expectedDay int) {
		tim := time.Date(2016, 5, day, 0, 0, 0, 0, time.Local)
		d := nextDay(tim, weekday)
		if d != time.Date(2016, 5, expectedDay, 0, 0, 0, 0, time.Local) {
			t.Errorf("nextDay() %v", weekday)
		}
	}

	test(16, time.Friday, 20)
	test(18, time.Monday, 23)
}

func TestSchedule_NextRunAfterTime(t *testing.T) {
	schedule := Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From:     nil,
		To:       nil,
	}
	refTime := time.Date(2016, 5, 16, 0, 0, 0, 0, time.Local)
	tim := schedule.NextRunAfterTime(refTime)
	if tim == nil || *tim != time.Date(2016, 5, 19, 8, 30, 0, 0, time.Local) {
		t.Error("Schedule#NextRunAfterTime 1")
	}
	refTime = time.Date(2016, 5, 20, 9, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	if tim == nil || *tim != time.Date(2016, 5, 20, 20, 0, 0, 0, time.Local) {
		t.Error("Schedule#NextRunAfterTime 2")
	}
}

func TestSchedule_NextRunTime(t *testing.T) {
	schedule := Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From:     nil,
		To:       nil,
	}
	if *schedule.NextRunTime() != *schedule.NextRunAfterTime(time.Now()) {
		t.Error("Schedule#NextRunTime")
	}
}