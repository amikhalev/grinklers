package logic

import (
	"sync"
	"time"

	"git.amikhalev.com/amikhalev/grinklers/sched"
	. "git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/Sirupsen/logrus"
)

// ProgItem is one item in a Program sequence
type ProgItem struct {
	Sec      Section
	Duration time.Duration
}

// ProgRunnerMsg represents a Message sent to a ProgRunner to tell it what to do
type ProgRunnerMsg int

const (
	prQuit ProgRunnerMsg = iota
	prCancel
	prRefresh
	prRun
)

// ProgSequence is a sequence of ProgItems
type ProgSequence []ProgItem

// ProgUpdateType represents the types of program updates that can happen
type ProgUpdateType int

const (
	ProgUpdateData ProgUpdateType = iota
	ProgUpdateRunning
)

// ProgUpdate represents an update that needs to be reflected about a Program
type ProgUpdate struct {
	Prog *Program
	Type ProgUpdateType
}

// Program represents a sprinklers program, which runs on a schedule and contains
// a sequence of sections to run.
type Program struct {
	ID         int
	Name       string
	Sequence   ProgSequence
	Sched      sched.Schedule
	Enabled    bool
	running    AtomicBool
	runner     chan ProgRunnerMsg
	UpdateChan chan<- ProgUpdate
	log        *logrus.Entry
	sync.Mutex // all fields should be accessed through this mutext
}

// NewProgram creates a new Program with the specified data
func NewProgram(name string, sequence []ProgItem, schedule sched.Schedule, enabled bool) *Program {
	runner := make(chan ProgRunnerMsg)
	return &Program{
		0, name, sequence, schedule, enabled,
		NewAtomicBool(false), runner, nil,
		Logger.WithField("program", name),
		sync.Mutex{},
	}
}

func (prog *Program) OnUpdate(t ProgUpdateType) {
	if prog.UpdateChan != nil {
		//prog.Debug("prog.onUpdate()")
		prog.UpdateChan <- ProgUpdate{
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
	prog.OnUpdate(ProgUpdateRunning)
	stop := func() {
		prog.running.Store(false)
		prog.OnUpdate(ProgUpdateRunning)
	}
	prog.Lock()
	seq := prog.Sequence
	prog.Unlock()
	seqLen := len(seq)
	runIds := make([]int32, seqLen)
	secDoneChans := make([]<-chan bool, seqLen)
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
		prog.Lock()
		if prog.Enabled {
			nextRun = prog.Sched.NextRunTime()
		} else {
			nextRun = nil
		}
		prog.Unlock()
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

func (prog *Program) Refresh() {
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
