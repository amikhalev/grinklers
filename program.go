package grinklers

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/amikhalev/grinklers/sched"
)

// ProgItem is one item in a Program sequence
type ProgItem struct {
	Sec      Section
	Duration time.Duration
}

// ProgItemJSON is the JSON representation of a ProgItem
type ProgItemJSON struct {
	Section int `json:"section"`
	// Duration of the program item in seconds
	Duration float64 `json:"duration"`
}

// ToProgItem converts a ProgItemJSON to a ProgItem
func (data *ProgItemJSON) ToProgItem(sections []Section) (pi *ProgItem, err error) {
	dur := time.Duration(data.Duration * float64(time.Second))
	if err = CheckRange(&data.Section, "section id", len(sections)); err != nil {
		err = fmt.Errorf("invalid program item section id: %v", err)
		return
	}
	pi = &ProgItem{sections[data.Section], dur}
	return
}

// ToJSON converts a ProgItem to a ProgItemJSON
func (pi *ProgItem) ToJSON(sections []Section) (data *ProgItemJSON, err error) {
	secID := -1
	for i, sec := range sections {
		if pi.Sec == sec {
			secID = i
		}
	}
	if secID == -1 {
		err = fmt.Errorf("the section of this program does not exist in the sections array")
		return
	}
	data = &ProgItemJSON{secID, pi.Duration.Seconds()}
	return
}

// ProgRunnerMsg represents a message sent to a ProgRunner to tell it what to do
type ProgRunnerMsg int

const (
	prQuit ProgRunnerMsg = iota
	prCancel
	prRefresh
	prRun
)

// ProgSequence is a sequence of ProgItems
type ProgSequence []ProgItem

// ProgSequenceJSON is a sequence of ProgItemJSONs
type ProgSequenceJSON []ProgItemJSON

// ToJSON converts a ProgSequence to a ProgSequenceJSON
func (seq ProgSequence) ToJSON(sections []Section) (seqj ProgSequenceJSON, err error) {
	seqj = make(ProgSequenceJSON, len(seq))
	var pi *ProgItemJSON
	for i := range seq {
		pi, err = seq[i].ToJSON(sections)
		if err != nil {
			return
		}
		seqj[i] = *pi
	}
	return
}

// ToSequence converts a ProgSequenceJSON to a ProgSequence
func (seqj ProgSequenceJSON) ToSequence(sections []Section) (seq ProgSequence, err error) {
	seq = make(ProgSequence, len(seqj))
	var pi *ProgItem
	for i := range seqj {
		pi, err = seqj[i].ToProgItem(sections)
		if err != nil {
			return
		}
		seq[i] = *pi
	}
	return
}

// ProgUpdateType represents the types of program updates that can happen
type ProgUpdateType int

const (
	ProgUpdateData ProgUpdateType = iota
	pupdateRunning
)

// ProgUpdate represents an update that needs to be reflected about a Program
type ProgUpdate struct {
	Prog *Program
	Type ProgUpdateType
}

// Program represents a sprinklers program, which runs on a schedule and contains
// a sequence of sections to run.
type Program struct {
	ID       int
	Name     string
	Sequence ProgSequence
	Sched    sched.Schedule
	Enabled  bool
	mutex    *sync.Mutex
	running  AtomicBool
	runner   chan ProgRunnerMsg
	OnUpdate chan<- ProgUpdate
	log      *logrus.Entry
}

// NewProgram creates a new Program with the specified data
func NewProgram(name string, sequence []ProgItem, schedule sched.Schedule, enabled bool) *Program {
	runner := make(chan ProgRunnerMsg)
	prog := &Program{
		0, name, sequence, schedule, enabled,
		&sync.Mutex{}, NewAtomicBool(false), runner, nil,
		Logger.WithField("program", name),
	}
	return prog
}

// ProgramJSON is the JSON representation of a Program
type ProgramJSON struct {
	ID       int              `json:"id"`
	Name     *string          `json:"name"`
	Sequence ProgSequenceJSON `json:"sequence"`
	Sched    *sched.Schedule  `json:"schedule"`
	Enabled  *bool            `json:"enabled"`
}

// NewProgramJSON creates a new ProgramJSON with the specified data
func NewProgramJSON(name *string, sequence ProgSequenceJSON, sched *sched.Schedule, enabled *bool) ProgramJSON {
	return ProgramJSON{
		0, name, sequence, sched, enabled,
	}
}

// ToProgram converts a ProgramJSON to a Program
func (p *ProgramJSON) ToProgram(sections []Section) (prog *Program, err error) {
	var (
		sequence []ProgItem
		schedule = sched.Schedule{}
		enabled  = false
	)
	if err = CheckNotNil(p.Name, "name"); err != nil {
		return
	}
	sequence, err = p.Sequence.ToSequence(sections)
	if err != nil {
		return
	}
	if p.Sched != nil {
		schedule = *p.Sched
	}
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	// id will be assigned later
	prog = NewProgram(*p.Name, sequence, schedule, enabled)
	return
}

// ToJSON converts a Program to a ProgramJSON
func (prog *Program) ToJSON(sections []Section) (data ProgramJSON, err error) {
	sequence, err := prog.Sequence.ToJSON(sections)
	if err != nil {
		return
	}
	data = ProgramJSON{prog.ID, &prog.Name, sequence, &prog.Sched, &prog.Enabled}
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
	}
}

func (prog *Program) run(cancel <-chan int, secRunner *SectionRunner) {
	if !prog.running.StoreIf(false, true) {
		prog.log.Info("program was started when already running")
		return
	}
	prog.log.Info("running program")
	prog.onUpdate(pupdateRunning)
	stop := func() {
		prog.running.Store(false)
		prog.onUpdate(pupdateRunning)
	}
	prog.lock()
	seq := prog.Sequence
	prog.unlock()
	seqLen := len(seq)
	runIds := make([]int32, seqLen)
	secDoneChans := make([]<-chan int, seqLen)
	for i, item := range seq {
		runIds[i], secDoneChans[i] = secRunner.RunSectionAsync(item.Sec, item.Duration)
	}
	for i := 0; i < seqLen; i++ {
		select {
		case <-secDoneChans[i]:
			continue
		case <-cancel:
			for j := seqLen - 1; j >= i; j-- {
				secRunner.CancelID(runIds[j])
			}
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
		// prog.log.WithField("delay", delay).Debug("runner waiting for command")
		select {
		case msg = <-prog.runner:
			// prog.log.WithField("cmd", msg).Debug("runner cmd")
			switch msg {
			case prQuit:
				cancel()
				prog.log.Debug("quitting program runner")
				return
			case prCancel:
				cancel()
			case prRefresh:
				continue
			case prRun:
				run()
			}
		case <-delay:
			run()
		}
	}
}

// Start starts the background goroutine which runs the program at the appropriate
// schedule
func (prog *Program) Start(secRunner *SectionRunner, wait *sync.WaitGroup) {
	if wait != nil {
		wait.Add(1)
	}
	go prog.start(secRunner, wait)
}

// Run runs the Program immediately
func (prog *Program) Run() {
	prog.runner <- prRun
}

// Cancel cancels running the Program
func (prog *Program) Cancel() {
	prog.runner <- prCancel
}

func (prog *Program) refresh() {
	prog.runner <- prRefresh
}

// Quit tells the background goroutine for this Program to stop
func (prog *Program) Quit() {
	prog.runner <- prQuit
}

// Running checks if the goroutine is currently running
func (prog *Program) Running() bool {
	return prog.running.Load()
}

// Update updates the data for this program based on the specified ProgramJSON, notifying
// the runner of any changes.
func (prog *Program) Update(data ProgramJSON, sections []Section) (err error) {
	prog.lock()
	defer prog.unlock()
	if data.Name != nil {
		prog.log.WithField("name", *data.Name).Debug("updating program name")
		prog.Name = *data.Name
	}
	if data.Sequence != nil {
		sequence, err := data.Sequence.ToSequence(sections)
		if err != nil {
			return err
		}
		prog.log.WithField("sequence", sequence).Debug("updating program sequence")
		prog.Sequence = sequence
	}
	if data.Sched != nil {
		prog.log.WithField("sched", *data.Sched).Debug("updating program sched")
		prog.Sched = *data.Sched
	}
	if data.Enabled != nil {
		prog.log.WithField("enabled", *data.Enabled).Debug("updating program enabled")
		prog.Enabled = *data.Enabled
	}
	prog.refresh()
	prog.onUpdate(ProgUpdateData)
	return
}

// ProgramsJSON represents multiple ProgramJSONs in a JSON array
type ProgramsJSON []ProgramJSON

// ToPrograms converts this ProgramsJSON to Programs
func (progs ProgramsJSON) ToPrograms(sections []Section) (programs []Program, err error) {
	programs = make([]Program, len(progs))
	var p *Program
	for i := range progs {
		p, err = progs[i].ToProgram(sections)
		if err != nil {
			return
		}
		p.ID = i
		programs[i] = *p
	}
	return
}

// ProgramsToJSON converts programs to ProgramsJSON
func ProgramsToJSON(programs []Program, sections []Section) (data ProgramsJSON, err error) {
	data = make(ProgramsJSON, len(programs))
	for i := range programs {
		data[i], err = programs[i].ToJSON(sections)
		if err != nil {
			return
		}
	}
	return
}
