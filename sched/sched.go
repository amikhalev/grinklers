package sched

import (
	"fmt"
	"time"
)

type TimeOfDay struct {
	Hour        int `json:"hour"`
	Minute      int `json:"minute"`
	Second      int `json:"second"`
	Millisecond int `json:"millisecond"`
}

func (tod *TimeOfDay) Duration() time.Duration {
	return time.Duration(tod.Hour)*time.Hour +
		time.Duration(tod.Minute)*time.Minute +
		time.Duration(tod.Second)*time.Second +
		time.Duration(tod.Millisecond)*time.Millisecond
}

func (t *TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d:%02d:%04d", t.Hour, t.Minute, t.Second, t.Millisecond)
}

type Date struct {
	Year  uint       `json:"year"`
	Month time.Month `json:"month"`
	Day   uint       `json:"day"`
}

type Schedule struct {
	Times    []TimeOfDay    `json:"times"`
	Weekdays []time.Weekday `json:"weekdays"`
	From     *Date          `json:"from"`
	To       *Date          `json:"to"`
}

func weeks(weeks int64) time.Duration {
	return time.Duration(weeks) * 24 * time.Hour
}

func nextDay(t time.Time, wd time.Weekday) time.Time {
	year, month, day := t.Date()
	timeWithDay := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	weekday := timeWithDay.Weekday()
	if weekday != wd {
		diff := wd - weekday
		if diff < 0 {
			diff += 7
		}
		timeWithDay = timeWithDay.Add(weeks(int64(diff)))
	}
	return timeWithDay
}

func (sched *Schedule) NextRunTime() *time.Time {
	return sched.NextRunAfterTime(time.Now())
}

func (sched *Schedule) NextRunAfterTime(timeReference time.Time) *time.Time {
	var nextRunTime *time.Time
	for _, weekday := range sched.Weekdays {
		timeWithDay := nextDay(timeReference, weekday)
		for _, tod := range sched.Times {
			time := timeWithDay.Add(tod.Duration())
			if time.Before(timeReference) {
				time = time.Add(weeks(1))
			}
			if nextRunTime == nil || nextRunTime.After(time) {
				nextRunTime = &time
			}
		}
	}
	return nextRunTime
}

var everyDay = []time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}