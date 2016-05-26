package main

import (
	"encoding/json"
	"github.com/amikhalev/grinklers/sched"
	"github.com/inconshreveable/log15"
	"time"
	"sync"
)

type ProgItem struct {
	Sec      Section
	Duration time.Duration
}

type progItemJson struct {
	Section  uint   `json:"section"`
	Duration string `json:"duration"`
}

func (pi *ProgItem) UnmarshalJSON(b []byte) (err error) {
	var data progItemJson
	if err = json.Unmarshal(b, &data); err == nil {
		var dur time.Duration
		dur, err = time.ParseDuration(data.Duration)
		if err != nil {
			return
		}
		*pi = ProgItem{&configData.Sections[data.Section], dur}
	}
	return
}

func (pi *ProgItem) MarshalJSON() (b []byte, err error) {
	data := progItemJson{0, pi.Duration.String()}
	b, err = json.Marshal(&data)
	return
}

type ProgRunnerMsg int

const (
	PR_QUIT ProgRunnerMsg = iota
	PR_CANCEL
	PR_REFRESH
	PR_RUN
)

type Program struct {
	Name     string
	Sequence []ProgItem
	Sched    sched.Schedule
	Enabled  bool
	mutex    *sync.Mutex
	running  bool
	runner   chan ProgRunnerMsg
	OnUpdate chan <- *Program
	log15.Logger
}

func NewProgram(name string, sequence []ProgItem, schedule sched.Schedule, enabled bool) Program {
	runner := make(chan ProgRunnerMsg)
	prog := Program{
		name, sequence, schedule, enabled,
		&sync.Mutex{}, false, runner, nil,
		logger.New("program", name),
	}
	return prog
}

type ProgramJson struct {
	Name     string         `json:"name"`
	Sequence []ProgItem     `json:"sequence"`
	Sched    sched.Schedule `json:"sched"`
	Enabled  bool           `json:"enabled"`
	Running  *bool          `json:"running,omitempty"`
}

func (prog *Program) UnmarshalJSON(b []byte) (err error) {
	var p ProgramJson
	if err = json.Unmarshal(b, &p); err == nil {
		*prog = NewProgram(p.Name, p.Sequence, p.Sched, p.Enabled)
	}
	return
}

func (prog *Program) MarshalJSON() (b []byte, err error) {
	p := ProgramJson{prog.Name, prog.Sequence, prog.Sched, prog.Enabled, &prog.running}
	b, err = json.Marshal(&p)
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
		prog.Debug("prog.onUpdate()")
		prog.OnUpdate <- prog
	} else {
		prog.Warn("OnUpdate is nil!", "onUpdate", prog.OnUpdate)
	}
}

func (prog *Program) run(cancel <-chan int) {
	if prog.Running() {
		return
	}
	prog.Info("running program")
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
		secDone := sectionRunner.RunSectionAsync(item.Sec, item.Duration)
		select {
		case <-secDone:
			continue
		case <-cancel:
			sectionRunner.CancelSection(item.Sec)
			prog.Info("program run cancelled")
			stop()
			return
		}
	}
	prog.Info("finished running program")
	stop()
}

func (prog *Program) start() {
	var (
		msg ProgRunnerMsg
		nextRun *time.Time
		delay   <-chan time.Time
	)
	cancelRun := make(chan int)
	run := func() {
		go prog.run(cancelRun)
	}
	for {
		if prog.Enabled {
			nextRun = prog.Sched.NextRunTime()
		} else {
			nextRun = nil
		}
		if nextRun != nil {
			dur := nextRun.Sub(time.Now())
			delay = time.After(dur)
			prog.Debug("program scheduled", "dur", dur)
		} else {
			delay = nil
			prog.Debug("program not scheduled", "enabled", prog.Enabled)
		}
		//prog.Debug("runner waiting for command", "delay", delay)
		select {
		case msg = <-prog.runner:
			prog.Debug("runner cmd", "msg", msg)
			switch msg {
			case PR_QUIT:
				cancelRun <- 0
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

func (prog *Program) Start() {
	go prog.start()
}

func (prog *Program) Run() {
	prog.runner <- PR_RUN
}

func (prog *Program) Cancel() {
	prog.runner <- PR_CANCEL
}

func (prog *Program) Refresh() {
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
