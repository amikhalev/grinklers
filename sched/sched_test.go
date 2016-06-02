package sched

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"encoding/json"
	"github.com/stretchr/testify/require"
)

func TestTimeOfDay(t *testing.T) {
	ass := assert.New(t)
	tod := TimeOfDay{1, 2, 3, 0}
	dur := tod.Duration()
	ass.Equal(float64(3 + 2 * 60 + 1 * (60 * 60)), dur.Seconds())
	ass.Equal("01:02:03:0000", tod.String())

	var tod2 TimeOfDay
	err := json.Unmarshal([]byte(`{"hour": 1, "minute": 2, "second": 3}`), &tod2)
	require.NoError(t, err)
	ass.Equal(tod, tod2)

	bytes, err := json.Marshal(&tod)
	require.NoError(t, err)
	ass.Equal([]byte(`{"hour":1,"minute":2,"second":3,"millisecond":0}`), bytes)
}

func TestDate(t *testing.T) {
	ass := assert.New(t)
	date := Date{2016, 6, 1}

	ass.Equal(date, DateFromTime(date.ToTime()))
	ass.Equal(date, date.WithResolvedYear())

	date2 := Date{0, 6, 1}
	year := time.Now().Year()
	ass.Equal(Date{year, 6, 1}, date2.WithResolvedYear())

	date3 := Date{2016, 6, 2}
	ass.Equal(false, date3.After(&date3))
	ass.Equal(false, date3.Before(&date3))
	ass.Equal(false, date.After(&date3))
	ass.Equal(true, date.Before(&date3))
	ass.Equal(true, date3.After(&date))
	ass.Equal(false, date3.Before(&date))

	ass.Equal(true, date.Before(&Date{2017, 1, 1}))
	ass.Equal(true, date.After(&Date{2015, 1, 1}))
	ass.Equal(true, date.Before(&Date{2016, 7, 1}))
	ass.Equal(true, date.After(&Date{2016, 5, 1}))
}

func TestNextDay(t *testing.T) {
	test := func(day int, weekday time.Weekday, expectedDay int) {
		tim := time.Date(2016, 5, day, 0, 0, 0, 0, time.Local)
		d := nextDay(tim, weekday)
		assert.Equal(t, time.Date(2016, 5, expectedDay, 0, 0, 0, 0, time.Local), d)
	}

	test(16, time.Friday, 20)
	test(18, time.Monday, 23)
}

func TestSchedule_NextRunAfterTime(t *testing.T) {
	req := require.New(t)
	ass := assert.New(t)
	schedule := Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From:     nil,
		To:       nil,
	}
	refTime := time.Date(2016, 5, 16, 0, 0, 0, 0, time.Local)
	tim := schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 5, 19, 8, 30, 0, 0, time.Local), *tim)

	refTime = time.Date(2016, 5, 20, 9, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 5, 20, 20, 0, 0, 0, time.Local), *tim)

	schedule = Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From:     &Date{2016, 5, 30},
		To:       &Date{2016, 6, 30},
	}
	refTime = time.Date(2016, 6, 1, 0, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 6, 2, 8, 30, 0, 0, time.Local), *tim)

	refTime = time.Date(2016, 5, 1, 0, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 6, 2, 8, 30, 0, 0, time.Local), *tim)

	refTime = time.Date(2016, 7, 1, 0, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	if !ass.Nil(tim) {
		ass.Fail("tim not nil", "tim: %v", *tim)
	}

	schedule = Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: EveryDay,
		From:     &Date{0, 12, 15},
		To:       &Date{0, 1, 15},
	}
	refTime = time.Date(2016, 11, 1, 0, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 12, 15, 8, 30, 0, 0, time.Local), *tim)

	refTime = time.Date(2017, 1, 1, 9, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2017, 1, 1, 20, 0, 0, 0, time.Local), *tim)

	refTime = time.Date(2016, 1, 30, 0, 0, 0, 0, time.Local)
	tim = schedule.NextRunAfterTime(refTime)
	req.NotNil(tim)
	ass.Equal(time.Date(2016, 12, 15, 8, 30, 0, 0, time.Local), *tim)
}

func TestSchedule_NextRunTime(t *testing.T) {
	ass := assert.New(t)
	schedule := Schedule{
		Times:    []TimeOfDay{TimeOfDay{8, 30, 0, 0}, TimeOfDay{20, 0, 0, 0}},
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From:     nil,
		To:       nil,
	}
	ass.Equal(*schedule.NextRunAfterTime(time.Now()), *schedule.NextRunTime())
}