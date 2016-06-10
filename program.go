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
		return
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

type ProgSequenceJSON []ProgItemJSON

func (seq ProgSequence) ToJSON(sections []Section) (seqj ProgSequenceJSON, err error) {
	seqj = make(ProgSequenceJSON, len(seq))
	var pi *ProgItemJSON
	for i, _ := range seq {
		pi, err = seq[i].ToJSON(sections)
		if err != nil {
			return
		}
		seqj[i] = *pi
	}
	return
}

func (seqj ProgSequenceJSON) ToSequence(sections []Section) (seq ProgSequence, err error) {
	seq = make(ProgSequence, len(seqj))
	var pi *ProgItem
	for i, _ := range seqj {
		pi, err = seqj[i].ToProgItem(sections)
		if err != nil {
			return
		}
		seq[i] = *pi
	}
	return
}

type ProgUpdateType int

const (
	PROG_UPDATE_DATA ProgUpdateType = iota
	PROG_UPDATE_RUNNING
)

type ProgUpdate struct {
	Prog *Program
	Type ProgUpdateType
}

type Program struct {
	Name     string
	Sequence ProgSequence
	Sched    Schedule
	Enabled  bool
	mutex    *sync.Mutex
	running  AtomicBool
	runner   chan ProgRunnerMsg
	OnUpdate chan<- ProgUpdate
	log      *logrus.Entry
}

func NewProgram(name string, sequence []ProgItem, schedule Schedule, enabled bool) *Program {
	runner := make(chan ProgRunnerMsg)
	prog := &Program{
		name, sequence, schedule, enabled,
		&sync.Mutex{}, NewAtomicBool(false), runner, nil,
		Logger.WithField("program", name),
	}
	return prog
}

type ProgramJSON struct {
	Name     *string           `json:"name"`
	Sequence *ProgSequenceJSON `json:"sequence"`
	Sched    *Schedule         `json:"sched"`
	Enabled  *bool             `json:"enabled"`
}

func NewProgramJSON(name string, sequence ProgSequenceJSON, sched *Schedule, enabled bool) ProgramJSON {
	return ProgramJSON{
		&name, &sequence, sched, &enabled,
	}
}

func (p *ProgramJSON) ToProgram(sections []Section) (prog *Program, err error) {
	var (
		sequence []ProgItem
		schedule = Schedule{}
		enabled  = false
	)
	if err = CheckNotNil(p.Name, "name"); err != nil {
		return
	}
	if err = CheckNotNil(p.Sequence, "sequence"); err != nil {
		return
	} else {
		sequence, err = p.Sequence.ToSequence(sections)
		if err != nil {
			return
		}
	}
	if p.Sched != nil {
		schedule = *p.Sched
	}
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	prog = NewProgram(*p.Name, sequence, schedule, enabled)
	return
}

func (prog *Program) ToJSON(sections []Section) (data ProgramJSON, err error) {
	sequence, err := prog.Sequence.ToJSON(sections)
	if err != nil {
		return
	}
	data = ProgramJSON{&prog.Name, &sequence, &prog.Sched, &prog.Enabled}
	return
}

func (prog *Program) lock() {
	prog.mutex.Lock()
}

func (prog *Program) unlock() {
	prog.mutex.Unlock()
}

func (prog *Program) onUpdate(t ProgUpdateType) {
	if prog.OnUpdate != nil {
		//prog.Debug("prog.onUpdate()")
		prog.OnUpdate <- ProgUpdate{
			Prog: prog, Type: t,
		}
	} else {
		prog.log.Warnf("OnUpdate is nil! :%v", prog.OnUpdate)
	}
}

func (prog *Program) run(cancel <-chan int, secRunner *SectionRunner) {
	if !prog.running.StoreIf(false, true) {
		prog.log.Info("program was started when already running")
		return
	}
	prog.log.Info("running program")
	prog.onUpdate(PROG_UPDATE_RUNNING)
	stop := func() {
		prog.running.Store(false)
		prog.onUpdate(PROG_UPDATE_RUNNING)
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

func (prog *Program) start(secRunner *SectionRunner, wait *sync.WaitGroup) {
	var (
		msg     ProgRunnerMsg
		nextRun *time.Time
		delay   <-chan time.Time
	)
	cancelRun := make(chan int)
	run := func() {
		go prog.run(cancelRun, secRunner)
	}
	cancel := func() {
		if prog.Running() {
			cancelRun <- 0
		}
	}
	if wait != nil {
		defer wait.Done()
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
		prog.log.WithField("delay", delay).Debug("runner waiting for command")
		select {
		case msg = <-prog.runner:
			prog.log.WithField("cmd", msg).Debug("runner cmd")
			switch msg {
			case PR_QUIT:
				cancel()
				prog.log.Debug("quitting runner")
				return
			case PR_CANCEL:
				cancel()
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

func (prog *Program) Start(secRunner *SectionRunner, wait *sync.WaitGroup) {
	if wait != nil {
		wait.Add(1)
	}
	go prog.start(secRunner, wait)
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

func (prog *Program) Running() bool {
	return prog.running.Load()
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
	prog.onUpdate(PROG_UPDATE_DATA)
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
