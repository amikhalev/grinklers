package main

import (
	"time"
	log "github.com/inconshreveable/log15"
	"encoding/json"
	"github.com/amikhalev/grinklers/sched"
)

type ProgItem struct {
	Section  uint
	Duration time.Duration
}

type progItemJson struct {
	Section  uint `json:"section"`
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
		*pi = ProgItem{data.Section, dur}
	}
	return
}

func (pi *ProgItem) MarshalJSON() (b []byte, err error) {
	data := progItemJson{pi.Section, pi.Duration.String()}
	b, err = json.Marshal(&data)
	return
}

type ProgRunnerMsg int

const (
	PR_QUIT ProgRunnerMsg = iota
	PR_STOP
	PR_REFRESH
	PR_RUN
)

type Program struct {
	Name     string
	Sequence []ProgItem
	Sched    sched.Schedule
	Enabled  bool
	running  bool
	runner   chan ProgRunnerMsg
	log.Logger
}

func NewProgram(name string, sequence []ProgItem, schedule sched.Schedule, enabled bool) Program {
	runner := make(chan ProgRunnerMsg)
	prog := Program{
		name, sequence, schedule, enabled,
		false, runner,
		log.New("program", name),
	}
	go prog.start()
	return prog
}

func (prog *Program) run(cancel <-chan int) {
	sections := configData.Sections
	prog.running = true
	prog.Info("running program")
	for _, item := range prog.Sequence {
		sec := sections[item.Section]
		secDone := sec.RunForAsync(item.Duration)
		select {
		case <-secDone:
			continue
		case <-cancel:
			sec.Cancel()
			log.Info("program run cancelled")
			return
		}
	}
	prog.Info("finished running program")
	prog.running = false
}

func (prog *Program) start() {
	var (
		msg ProgRunnerMsg; nextRun *time.Time; delay <-chan time.Time
	)
	cancelRun := make(chan int)
	run := func() {
		go prog.run(cancelRun)
	}
	Loop:
	for {
		if prog.Enabled {
			nextRun = prog.Sched.NextRunTime()
		} else {
			nextRun = nil
		}
		if nextRun != nil {
			dur := nextRun.Sub(time.Now())
			delay = time.After(dur)
			prog.Debug("progRunner(): waiting", "dur", dur)
		} else {
			delay = nil
			prog.Debug("progRunner(): not running. waiting for refresh", "enabled", prog.Enabled, "nextRun", nextRun)
		}
		select {
		case msg = <-prog.runner:
			switch msg {
			case PR_QUIT:
				cancelRun <- 0
				break Loop
			case PR_STOP:
				cancelRun <- 0
			case PR_REFRESH:
				continue Loop
			case PR_RUN:
				run()
			}
		case <-delay:
			run()
		}
	}
}

type programJson struct {
	Name     string `json:"name"`
	Sequence []ProgItem `json:"sequence"`
	Sched    sched.Schedule `json:"sched"`
	Enabled  bool `json:"enabled"`
	Running  *bool `json:"running,omitempty"`
}

func (prog *Program) UnmarshalJSON(b []byte) (err error) {
	var p programJson;
	if err = json.Unmarshal(b, &p); err == nil {
		*prog = NewProgram(p.Name, p.Sequence, p.Sched, p.Enabled)
	}
	return
}

func (prog *Program) MarshalJSON() (b []byte, err error) {
	p := programJson{prog.Name, prog.Sequence, prog.Sched, prog.Enabled, &prog.running};
	b, err = json.Marshal(&p)
	return
}

func (prog *Program) Run() {
	prog.runner <- PR_RUN
}

func (prog *Program) Stop() {
	prog.runner <- PR_STOP
}

func (prog *Program) Refresh() {
	prog.runner <- PR_REFRESH
}

func (prog *Program) Kill() {
	prog.runner <- PR_QUIT
}
