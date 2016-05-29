package grinklers

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/amikhalev/grinklers/sched"
)

type ProgItem struct {
	Sec      Section
	Duration time.Duration
}

type ProgItemJSON struct {
	Section  int    `json:"section"`
	Duration string `json:"duration"`
}

func (data *ProgItemJSON) ToProgItem(sections []Section) (pi *ProgItem, err error) {
	var dur time.Duration
	dur, err = time.ParseDuration(data.Duration)
	if err != nil {
		err = fmt.Errorf("error parsing ProgItem duration: %v", err)
		return
	}
	if err = CheckRange(&data.Section, "section id", len(sections)); err != nil {
		err = fmt.Errorf("invalid program item section id: %v", err)
	}
	pi = &ProgItem{sections[data.Section], dur}
	return
}

func (pi *ProgItem) ToJSON(sections []Section) (data *ProgItemJSON, err error) {
	secId := -1
	for i, sec := range sections {
		if pi.Sec == sec {
			secId = i
		}
	}
	if secId == -1 {
		err = fmt.Errorf("the section of this program does not exist in the sections array")
	}
	data = &ProgItemJSON{secId, pi.Duration.String()}
	return
}

type ProgRunnerMsg int

const (
	PR_QUIT ProgRunnerMsg = iota
	PR_CANCEL
	PR_REFRESH
	PR_RUN
)

type ProgSequence []ProgItem

type Program struct {
	Name     string
	Sequence ProgSequence
	Sched    Schedule
	Enabled  bool
	mutex    *sync.Mutex
	running  bool
	runner   chan ProgRunnerMsg
	OnUpdate chan<- *Program
	log      *logrus.Entry
}

func NewProgram(name string, sequence []ProgItem, schedule Schedule, enabled bool) *Program {
	runner := make(chan ProgRunnerMsg)
	prog := &Program{
		name, sequence, schedule, enabled,
		&sync.Mutex{}, false, runner, nil,
		Logger.WithField("program", name),
	}
	return prog
}

type ProgSequenceJSON []ProgItemJSON

func (seq ProgSequenceJSON) ToSequence(sections []Section) (sequence ProgSequence, err error) {
	sequence = make(ProgSequence, len(seq))
	var pi *ProgItem
	for i, _ := range seq {
		pi, err = seq[i].ToProgItem(sections)
		if err != nil {
			return
		}
		sequence[i] = *pi
	}
	return
}

type ProgramJSON struct {
	Name     *string          `json:"name"`
	Sequence ProgSequenceJSON `json:"sequence"`
	Sched    *Schedule        `json:"sched"`
	Enabled  *bool            `json:"enabled"`
	Running  *bool            `json:"running,omitempty"`
}

func (p *ProgramJSON) ToProgram(sections []Section) (prog *Program, err error) {
	if err = CheckNotNil(p.Name, "name"); err != nil {
		return
	}
	var sequence []ProgItem
	if err = CheckNotNil(p.Sequence, "sequence"); err != nil {
		return
	} else {
		sequence, err = p.Sequence.ToSequence(sections)
		if err != nil {
			return
		}
	}
	if err = CheckNotNil(p.Sched, "sched"); err != nil {
		return
	}
	if err = CheckNotNil(p.Enabled, "enabled"); err != nil {
		return
	}
	prog = NewProgram(*p.Name, sequence, *p.Sched, *p.Enabled)
	return
}

func (prog *Program) ToJSON(sections []Section) (data ProgramJSON, err error) {
	sequence := make([]ProgItemJSON, len(prog.Sequence))
	var pi *ProgItemJSON
	for i, _ := range prog.Sequence {
		pi, err = prog.Sequence[i].ToJSON(sections)
		if err != nil {
			return
		}
		sequence[i] = *pi
	}
	data = ProgramJSON{&prog.Name, sequence, &prog.Sched, &prog.Enabled, &prog.running}
	return
}

func (prog *Program) lock() {
	prog.mutex.Lock()
}

func (prog *Program) unlock() {
	prog.mutex.Unlock()
}

func (prog *Program) onUpdate() {
	if prog.OnUpdate != nil {
		//prog.Debug("prog.onUpdate()")
		prog.OnUpdate <- prog
	} else {
		prog.log.Warnf("OnUpdate is nil! :%v", prog.OnUpdate)
	}
}

func (prog *Program) run(cancel <-chan int, secRunner *SectionRunner) {
	if prog.Running() {
		prog.log.Info("program was started when already running")
		return
	}
	prog.log.Info("running program")
	prog.setRunning(true)
	prog.onUpdate()
	stop := func() {
		prog.setRunning(false)
		prog.onUpdate()
	}
	prog.lock()
	seq := prog.Sequence
	prog.unlock()
	for _, item := range seq {
		secDone := secRunner.RunSectionAsync(item.Sec, item.Duration)
		select {
		case <-secDone:
			continue
		case <-cancel:
			secRunner.CancelSection(item.Sec)
			prog.log.Info("program run cancelled")
			stop()
			return
		}
	}
	prog.log.Info("finished running program")
	stop()
}

func (prog *Program) start(secRunner *SectionRunner) {
	var (
		msg     ProgRunnerMsg
		nextRun *time.Time
		delay   <-chan time.Time
	)
	cancelRun := make(chan int)
	run := func() {
		go prog.run(cancelRun, secRunner)
	}
	for {
		prog.lock()
		if prog.Enabled {
			nextRun = prog.Sched.NextRunTime()
		} else {
			nextRun = nil
		}
		prog.unlock()
		if nextRun != nil {
			dur := nextRun.Sub(time.Now())
			delay = time.After(dur)
			prog.log.WithFields(logrus.Fields{"nextRun": nextRun, "waitDuration": dur}).Debug("program scheduled")
		} else {
			delay = nil
			prog.log.WithFields(logrus.Fields{"enabled": prog.Enabled}).Debug("program not scheduled")
		}
		//prog.Debug("runner waiting for command", "delay", delay)
		select {
		case msg = <-prog.runner:
			//prog.Debug("runner cmd", "msg", msg)
			switch msg {
			case PR_QUIT:
				cancelRun <- 0
				prog.log.Debug("quitting runner")
				return
			case PR_CANCEL:
				cancelRun <- 0
			case PR_REFRESH:
				continue
			case PR_RUN:
				run()
			}
		case <-delay:
			run()
		}
	}
}

func (prog *Program) Start(secRunner *SectionRunner) {
	go prog.start(secRunner)
}

func (prog *Program) Run() {
	prog.runner <- PR_RUN
}

func (prog *Program) Cancel() {
	prog.runner <- PR_CANCEL
}

func (prog *Program) refresh() {
	prog.runner <- PR_REFRESH
}

func (prog *Program) Quit() {
	prog.runner <- PR_QUIT
}

func (prog *Program) setRunning(running bool) {
	prog.lock()
	defer prog.unlock()
	prog.running = running
}

func (prog *Program) Running() bool {
	prog.lock()
	defer prog.unlock()
	return prog.running
}

func (prog *Program) Update(data ProgramJSON, sections []Section) (err error) {
	prog.lock()
	if data.Name != nil {
		prog.Name = *data.Name
	}
	if data.Sequence != nil {
		sequence, err := data.Sequence.ToSequence(sections)
		if err != nil {
			return err
		}
		prog.Sequence = sequence
	}
	if data.Sched != nil {
		prog.Sched = *data.Sched
	}
	if data.Enabled != nil {
		prog.Enabled = *data.Enabled
	}
	prog.unlock()
	prog.refresh()
	prog.onUpdate()
	return
}

type ProgramsJSON []ProgramJSON

func (progs ProgramsJSON) ToPrograms(sections []Section) (programs []Program, err error) {
	programs = make([]Program, len(progs))
	var p *Program
	for i, _ := range progs {
		p, err = progs[i].ToProgram(sections)
		if err != nil {
			return
		}
		programs[i] = *p
	}
	return
}

func ProgramsToJSON(programs []Program, sections []Section) (data ProgramsJSON, err error) {
	data = make(ProgramsJSON, len(programs))
	for i, _ := range programs {
		data[i], err = programs[i].ToJSON(sections)
		if err != nil {
			return
		}
	}
	return
}
