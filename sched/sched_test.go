package sched

import (
	"testing"
	"time"
)

func TestTimeOfDay(t *testing.T) {
	tod := TimeOfDay{1, 1, 1}
	dur := tod.Duration()
	if dur.Seconds() != 1 + 60 + 3600 {
		t.Error("TimeOfDay#Duration")
	}
	if tod.String() != "01:01:01" {
		t.Errorf("TimeOfDay#String() = %s", dur.String())
	}
}

func TestNextDay(t *testing.T) {
	test := func (day int, weekday time.Weekday, expectedDay int) {
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
		Times: []TimeOfDay{ TimeOfDay{8, 30, 0}, TimeOfDay{20, 0, 0} },
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From: nil,
		To: nil,
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
		Times: []TimeOfDay{ TimeOfDay{8, 30, 0}, TimeOfDay{20, 0, 0} },
		Weekdays: []time.Weekday{time.Thursday, time.Friday},
		From: nil,
		To: nil,
	}
	if *schedule.NextRunTime() != *schedule.NextRunAfterTime(time.Now()) {
		t.Error("Schedule#NextRunTime")
	}
}

func TestNewPattern(t *testing.T) {
	defer func (){
		if r := recover(); r == nil {
			t.Error("newPattern did not panic on invalid pattern")
		}
	}()
	newPattern("asdf", "[")
}

func assertTokenType(t *testing.T, tok token, ty TokenType) {
	if tok.ty != ty {
		t.Errorf("expected token %v, got %v", ty, tok.ty)
	}
}

func assertTokenContents(t *testing.T, tok token, contents string) {
	if contents != string(tok.Text()) {
		t.Errorf("expected contents %s, got %s", contents, tok.Text())
	}
}

func TestTokenize(t *testing.T) {
	tokens, err := tokenize([]byte("1234567890 / : At Am Pm On And Also Through " +
	"Thru - From Starting To Until Mon Monday Tue Tuesday Wed Wednesday " +
	"Thu Thur Thursday Fri Friday Sat Saturday Sun Sunday "))
	if err != nil {
		t.Errorf("tokenize returned err: %v", err); return
	}
	assertTokenType(t, tokens[0], INT)
	assertTokenContents(t, tokens[0], "1234567890")
	assertTokenType(t, tokens[1], SLASH)
	assertTokenType(t, tokens[2], COLON)
	assertTokenType(t, tokens[3], AT)
	assertTokenType(t, tokens[4], AM)
	assertTokenType(t, tokens[5], PM)
	assertTokenType(t, tokens[6], ON)
	assertTokenType(t, tokens[7], AND)
	assertTokenType(t, tokens[8], AND)
	assertTokenType(t, tokens[9], THROUGH)
	assertTokenType(t, tokens[10], THROUGH)
	assertTokenType(t, tokens[11], THROUGH)
	assertTokenType(t, tokens[12], FROM)
	assertTokenType(t, tokens[13], FROM)
	assertTokenType(t, tokens[14], TO)
	assertTokenType(t, tokens[15], TO)
	assertTokenType(t, tokens[16], MON)
	assertTokenType(t, tokens[17], MON)
	assertTokenType(t, tokens[18], TUE)
	assertTokenType(t, tokens[19], TUE)
	assertTokenType(t, tokens[20], WED)
	assertTokenType(t, tokens[21], WED)
	assertTokenType(t, tokens[22], THUR)
	assertTokenType(t, tokens[23], THUR)
	assertTokenType(t, tokens[24], THUR)
	assertTokenType(t, tokens[25], FRI)
	assertTokenType(t, tokens[26], FRI)
	assertTokenType(t, tokens[27], SAT)
	assertTokenType(t, tokens[28], SAT)
	assertTokenType(t, tokens[29], SUN)
	assertTokenType(t, tokens[30], SUN)
	assertTokenType(t, tokens[31], EOF)
}

func TestScheduleParser_Parse(t *testing.T) {
	scheduleStr := "At 12 am and 9:0:0 pm on mon, tue-thur and fri from 4/20 until 12/1"
	parser := ScheduleParser{}
	_, err := parser.Parse([]byte(scheduleStr))
	if err != nil {
		t.Errorf("unexpected schedule parse error: %v", err)
		return
	}
}