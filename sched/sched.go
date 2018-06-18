package sched

import (
	"fmt"
	"time"
)

var EveryDay = []time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}

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

func (tod *TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d:%02d:%04d", tod.Hour, tod.Minute, tod.Second, tod.Millisecond)
}

type Date struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Day   int        `json:"day"`
}

func DateFromTime(t time.Time) Date {
	return Date{t.Year(), t.Month(), t.Day()}
}

func (d *Date) ToTime() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.Local)
}

func (d *Date) WithResolvedYearNow() Date {
	return d.WithResolvedYear(time.Now())
}

func (d *Date) WithResolvedYear(refTime time.Time) Date {
	if d.Year == 0 {
		return Date{refTime.Year(), d.Month, d.Day}
	}
	return *d
}

func (d *Date) After(d2 *Date) bool {
	if d.Year > d2.Year {
		return true
	} else if d.Year == d2.Year {
		if d.Month > d2.Month {
			return true
		} else if d.Month == d2.Month {
			if d.Day > d2.Day {
				return true
			}
		}
	}
	return false
}

func (d *Date) Before(d2 *Date) bool {
	if *d == *d2 {
		return false
	} else if d.After(d2) {
		return false
	} else {
		return true
	}
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
	var (
		nextRunTime *time.Time
		to          *Date
		from        *Date
	)
	if sched.To != nil {
		to = &Date{}
		*to = sched.To.WithResolvedYear(timeReference)
	}
	if sched.From != nil {
		from = &Date{}
		*from = sched.From.WithResolvedYear(timeReference)
		if to != nil && from.After(to) {
			refDate := DateFromTime(timeReference)
			if (refDate.Before(to) && refDate.Before(from)) ||
				(refDate.After(to) && refDate.After(from)) {
				from.Year--
			} else {
				to.Year++
			}
		}
		if from.ToTime().After(timeReference) {
			// log.Printf("timeRef: %v, from: %v, to: %v", timeReference, from, to)
			timeReference = from.ToTime()
		}
	}
	for _, weekday := range sched.Weekdays {
		timeWithDay := nextDay(timeReference, weekday)
		for _, tod := range sched.Times {
			tim := timeWithDay.Add(tod.Duration())
			if tim.Before(timeReference) {
				tim = tim.Add(weeks(1))
			}
			if to != nil && tim.After(to.ToTime()) {
				// log.Printf("rejecting %v because after to: %v", tim, to)
				continue
			}
			if nextRunTime == nil || nextRunTime.After(tim) {
				nextRunTime = &tim
			}
		}
	}
	return nextRunTime
}
