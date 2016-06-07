package sched

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseError(t *testing.T) {
	ass := assert.New(t)
	bytes := []byte("hello 1234")

	ass.NotPanics(func() {
		newParseError("asdf", bytes, 0, 10)
	})
	ass.Panics(func() {
		newParseError("", bytes, -1, 10)
	})
	ass.Panics(func() {
		newParseError("", bytes, 11, 12)
	})
	ass.Panics(func() {
		newParseError("", bytes, 0, 11)
	})
	ass.Panics(func() {
		newParseError("", bytes, 0, -1)
	})
	ass.Panics(func() {
		newParseError("", bytes, 4, 3)
	})
	err := newParseError("asdf", bytes, 5, 8)
	ass.Equal("asdf: 'hello« 12»34'", err.Error())
	err = newParseError("asdf", bytes, 0, 10)
	ass.Equal("asdf: '«hello 1234»'", err.Error())
	err = newParseError("asdf", bytes, 5, 5)
	ass.Equal("asdf: 'hello«» 1234'", err.Error())
}

func TestNewPattern(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("newPattern did not panic on invalid pattern")
		}
	}()
	newPattern("asdf", "[")
}

func assertTokenType(t *testing.T, tok token, ty TokenType) {
	assert.Equal(t, ty, tok.ty, "token type not what expected")
}

func TestTokenize(t *testing.T) {
	tokens, err := tokenize([]byte("1234567890 / : At Am Pm On And Also Through " +
		"Thru - From Starting To Until Mon Monday Tue Tuesday Wed Wednesday " +
		"Thu Thur Thursday Fri Friday Sat Saturday Sun Sunday "))
	if err != nil {
		t.Errorf("tokenize returned err: %v", err)
		return
	}
	assertTokenType(t, tokens[0], INT)
	assert.Equal(t, "1234567890", tokens[0].Text())
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

func TestToken_String(t *testing.T) {
	bytes := []byte("AT SUNDAY")
	tok := token{AT, 0, 2, bytes}
	assert.Equal(t, "AT(AT)", tok.String())
	tok = token{SUN, 3, 9, bytes}
	assert.Equal(t, "SUN(SUNDAY)", tok.String())
}

var noErrorStrs = []string{
	"At 12 am and 9:0:0 pm on mon, tue-thur and fri from 4/20 until 12/1",
	"At 12 On Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday",
	"At 12",
	"At 12 on fri-mon",
	"at 12 from 12/01/00",
	"at 12 to 12/01/1900",
	"at 12 from 12/01/2000",
}

var errorStrs = []string{
	"At 12 On 12",
	"At 12 On",
	"At 12 On Monday Tuesday",
	"At 12 from 5/At",
	"At 12 from 5/12/at",
	"at 12 and",
	"on monday",
	"at 12 to at",
	"at 12:at",
	"at 12:1:at",
	"at 12 on mon-at",
	"at asdf",
}

func TestScheduleParser_Parse(t *testing.T) {
	parser := ScheduleParser{}

	for _, str := range noErrorStrs {
		_, err := parser.Parse([]byte(str))
		require.NoError(t, err)
	}

	for _, str := range errorStrs {
		_, err := parser.Parse([]byte(str))
		require.Error(t, err)
	}

	parser.input = []byte("at")
	parser.tokenize()
	_, err := parser.parseWeekdays()
	require.Error(t, err)
}
