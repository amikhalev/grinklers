package sched

import (
	"time"
	"fmt"
	"regexp"
	"strconv"
)

type ParseError struct {
	ErrorStr string
	Input    []byte
	Start    int
	End      int
}

func newParseError(errorString string, input []byte, start int, end int) *ParseError {
	lenInput := len(input)
	if start < 0 || start > lenInput {
		panic(fmt.Sprintf("ParseError start index out of range: 0 < %d < %d", start, lenInput))
	}
	if end < 0 || end > lenInput {
		panic(fmt.Sprintf("ParseError end index out of range: 0 < %d < %d", end, lenInput))
	}
	if start > end {
		panic(fmt.Sprintf("ParseError start index after end: %d > %d", start, end))
	}
	return &ParseError{errorString, input, start, end}
}

func (err *ParseError) Error() string {
	return fmt.Sprintf("%s: '%s«%s»%s'", err.ErrorStr, err.Input[:err.Start], err.Input[err.Start:err.End], err.Input[err.End:])
}

type TokenType int

const (
	ILLEGAL TokenType = iota
	EOF
	WS

	INT

	SLASH
	COLON

	AT
	AM
	PM
	ON
	AND
	THROUGH
	FROM
	TO

	MON
	TUE
	WED
	THUR
	FRI
	SAT
	SUN
)

func (t TokenType) String() string {
	return scheduleTokens[t].Name
}

type TokenPattern struct {
	Name  string
	Regex *regexp.Regexp
}

func newPattern(name string, regexStr string) TokenPattern {
	regex, err := regexp.Compile("(?i)" + regexStr)
	if err != nil {
		panic(fmt.Sprintf("error compiling token regexp: %v", err))
	}
	return TokenPattern{name, regex}
}

var scheduleTokens = createScheduleTokens()

func createScheduleTokens() map[TokenType]TokenPattern {
	tokens := make(map[TokenType]TokenPattern)

	tokens[EOF] = newPattern("EOF", "^$")
	tokens[WS] = newPattern("WS", "^\\s+")
	tokens[INT] = newPattern("INT", "^\\d+")
	tokens[SLASH] = newPattern("SLASH", "^/")
	tokens[COLON] = newPattern("COLON", "^:")
	tokens[AT] = newPattern("AT", "^at")
	tokens[AM] = newPattern("AM", "^am")
	tokens[PM] = newPattern("PM", "^pm")
	tokens[ON] = newPattern("ON", "^on")
	tokens[AND] = newPattern("AND", "^(and|also|,)")
	tokens[THROUGH] = newPattern("THROUGH", "^(through|thru|\\-)")
	tokens[FROM] = newPattern("FROM", "^(from|starting)")
	tokens[TO] = newPattern("TO", "^(to|until)")
	tokens[MON] = newPattern("MON", "^mon(day)?")
	tokens[TUE] = newPattern("TUE", "^tue(sday)?")
	tokens[WED] = newPattern("WED", "^wed(nesday)?")
	tokens[THUR] = newPattern("THUR", "^thu(r(sday)?)?")
	tokens[FRI] = newPattern("FRI", "^fri(day)?")
	tokens[SAT] = newPattern("SAT", "^sat(urday)?")
	tokens[SUN] = newPattern("SUN", "^sun(day)?")

	return tokens
}

type token struct {
	ty    TokenType
	start int
	end   int
	input []byte
}

func (t token) Len() int {
	return t.end - t.start
}

func (t token) Text() string {
	return string(t.input[t.start:t.end])
}

func (t token) String() string {
	return fmt.Sprintf("%v(%s)", t.ty, t.Text())
}

func tokenize(input []byte) (tokens []token, err error) {
	var matches []token
	pos := 0
	for pos < len(input) {
		matches = nil
		for tok, pat := range scheduleTokens {
			m := pat.Regex.FindIndex(input[pos:])
			if m != nil {
				matches = append(matches, token{tok, pos + m[0], pos + m[1], input})
			}
		}
		var candidate *token
		for _, match := range matches {
			if candidate == nil {
				if match.Len() > 0 {
					candidate = &match
				}
			} else {
				if match.Len() > candidate.Len() {
					candidate = &match
				}
			}
		}
		if candidate == nil {
			err = newParseError("could not find token matching", input, pos, len(input))
			return
		} else {
			if candidate.ty != WS {
				tokens = append(tokens, *candidate)
			}
		}
		pos = candidate.end
	}
	tokens = append(tokens, token{EOF, pos, pos, input})
	return
}

type ScheduleParser struct {
	input   []byte
	tokens  []token
	nextTok int
}

func (p *ScheduleParser) Parse(input []byte) (sched *Schedule, err error) {
	p.input = input
	p.nextTok = 0
	err = p.tokenize()
	if err != nil {
		return
	}
	sched, err = p.parseSchedule()
	if err != nil {
		return
	}
	if p.accept(EOF) == nil {
		err = newParseError("tokens left over at end of schedule", p.input, p.tokens[p.nextTok].start, len(p.input))
		return
	}
	return
}

func (p *ScheduleParser) tokenize() (err error) {
	p.tokens, err = tokenize(p.input)
	return
}

func (p *ScheduleParser) peek() *token {
	return &p.tokens[p.nextTok]
}

func (p *ScheduleParser) nextIs(ty TokenType) bool {
	return p.peek().ty == ty
}

func (p *ScheduleParser) accept(ty TokenType) *token {
	tok := p.peek()
	if tok.ty != ty {
		return nil
	}
	p.nextTok++
	return tok
}

func (p *ScheduleParser) expect(ty TokenType) (tok *token, err error) {
	tok = p.accept(ty)
	if tok == nil {
		tok := p.peek()
		err = newParseError(fmt.Sprintf("expected token %v, got %v", ty, tok.ty), p.input, tok.start, tok.end)
	}
	return
}

func (p *ScheduleParser) peek2() *token {
	if p.nextTok + 1 >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.nextTok + 1]
}

func (p *ScheduleParser) nextIs2(ty TokenType) bool {
	return p.peek2().ty == ty
}

func (p *ScheduleParser) parseSchedule() (sched *Schedule, err error) {
	if _, err = p.expect(AT); err != nil {
		return
	}
	var (
		times []TimeOfDay
		tim   *TimeOfDay
	)
	for true {
		tim, err = p.parseTimeOfDay()
		if err != nil {
			return
		}
		times = append(times, *tim)
		if p.nextIs(AND) && !p.nextIs2(AT) {
			p.accept(AND)
		} else {
			break
		}
	}
	var weekdays []time.Weekday
	if p.nextIs(ON) {
		weekdays, err = p.parseWeekdays()
		if err != nil {
			return
		}
	} else {
		weekdays = everyDay
	}
	var (
		from *Date
		to   *Date
	)
	if p.accept(FROM) != nil {
		from, err = p.parseDate()
		if err != nil {
			return
		}
	}
	if p.accept(TO) != nil {
		to, err = p.parseDate()
		if err != nil {
			return
		}
	}
	sched = &Schedule{times, weekdays, from, to}
	return
}

func (p *ScheduleParser) parseTimeOfDay() (t *TimeOfDay, err error) {
	hours, minutes, seconds := 0, 0, 0
	hours, err = p.parseInt()
	if err != nil {
		return
	}
	if p.accept(COLON) != nil {
		minutes, err = p.parseInt()
		if err != nil {
			return
		}
		if p.accept(COLON) != nil {
			seconds, err = p.parseInt()
			if err != nil {
				return
			}
		}
	}
	if p.accept(AM) != nil {
		if hours == 12 {
			hours = 0
		}
	} else if p.accept(PM) != nil {
		if hours != 12 {
			hours += 12
		}
	}
	hours %= 24
	return &TimeOfDay{hours, minutes, seconds, 0}, nil
}

func (p *ScheduleParser) parseInt() (i int, err error) {
	var tok *token
	tok, err = p.expect(INT)
	if err != nil {
		return
	}
	i, err = strconv.Atoi(string(tok.Text()))
	return
}

func (p *ScheduleParser) parseWeekdays() (weekdays []time.Weekday, err error) {
	_, err = p.expect(ON)
	if err != nil {
		return
	}
	var weekday time.Weekday
	for true {
		weekday, err = p.parseWeekday()
		if err != nil {
			return
		}
		if p.accept(THROUGH) != nil {
			var throughDay time.Weekday
			throughDay, err = p.parseWeekday()
			if err != nil {
				return
			}
			for day := weekday; day != throughDay; {
				weekdays = append(weekdays, day)
				day++
				if day > time.Saturday {
					day = time.Sunday
				}
			}
			weekdays = append(weekdays, throughDay)
		} else {
			weekdays = append(weekdays, weekday)
		}
		if p.nextIs(AND) && !p.nextIs2(AT) {
			p.accept(AND)
		} else {
			break
		}
	}
	return
}

func (p *ScheduleParser) parseWeekday() (weekday time.Weekday, err error) {
	if p.accept(MON) != nil {
		weekday = time.Monday
	} else if p.accept(TUE) != nil {
		weekday = time.Tuesday
	} else if p.accept(WED) != nil {
		weekday = time.Wednesday
	} else if p.accept(THUR) != nil {
		weekday = time.Thursday
	} else if p.accept(FRI) != nil {
		weekday = time.Friday
	} else if p.accept(SAT) != nil {
		weekday = time.Saturday
	} else if p.accept(SUN) != nil {
		weekday = time.Sunday
	} else {
		tok := p.peek()
		err = newParseError(fmt.Sprintf("Expected day of week, got %v", p.peek().ty), p.input, tok.start, tok.end)
	}
	return
}

func (p *ScheduleParser) parseDate() (date *Date, err error) {
	year, month, day := 0, 0, 0
	month, err = p.parseInt()
	if err != nil {
		return
	}
	_, err = p.expect(SLASH)
	day, err = p.parseInt()
	if err != nil {
		return
	}
	if p.accept(SLASH) != nil {
		year, err = p.parseInt()
		if err != nil {
			return
		}
		if year < 100 {
			year += (time.Now().Year() / 100) * 100
		}
	}
	return &Date{uint(year), time.Month(month), uint(day)}, nil
}

