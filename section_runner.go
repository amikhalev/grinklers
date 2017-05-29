package grinklers

import (
	"fmt"
	"time"

	"sync"

	"github.com/Sirupsen/logrus"
)

// SectionRun is a single run of a section for a duration that is either queued, or currently running
type SectionRun struct {
	// Sec is the section that should be run
	Sec Section
	// Duration is the duration the section is run for
	Duration time.Duration
	// Done is a chan that a value is sent on when the section is done running
	Done chan<- int
	// StartTime is the time the section started running, or nil if the section is still queued
	StartTime *time.Time
}

func (sr *SectionRun) String() string {
	return fmt.Sprintf("{'%s' for %v}", sr.Sec.Name(), sr.Duration)
}

// SrQueue is a queue for SectionRuns. It is implemented as a circular buffer that doubles in length when it fills up
type SrQueue struct {
	items []*SectionRun
	head  int
	tail  int
}

// Format implements Formatter for SrQueue
func (q SrQueue) Format(f fmt.State, c rune) {
	fmt.Fprint(f, "[")
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if i != q.head {
			fmt.Fprint(f, ", ")
		}
		if q.items[i] != nil {
			fmt.Fprint(f, q.items[i])
		}
	}
	fmt.Fprint(f, "]")
}

var _ fmt.Formatter = (*SrQueue)(nil)

func newSRQueue(size int) SrQueue {
	return SrQueue{
		make([]*SectionRun, size),
		0, 0,
	}
}

// Push adds an item to the end of the SrQueue, expanding it if necessary
func (q *SrQueue) Push(item *SectionRun) {
	q.items[q.tail] = item
	itemsLen := len(q.items)
	q.tail = (q.tail + 1) % itemsLen
	if q.tail == q.head {
		// if queue is full, double storage size
		newItems := make([]*SectionRun, len(q.items)*2)
		copy(newItems, q.items[q.head:])
		copy(newItems[itemsLen-q.head:], q.items[:q.head])
		q.head = 0
		q.tail = itemsLen
		q.items = newItems
	}
}

// Pop pops the first item off the SrQueue
func (q *SrQueue) Pop() *SectionRun {
	if q.head == q.tail {
		return nil
	}
	item := q.items[q.head]
	q.head = (q.head + 1) % len(q.items)
	if item == nil {
		return q.Pop()
	}
	return item
}

// Len gets the current number of items in the SrQueue
func (q *SrQueue) Len() int {
	count := 0
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			count++
		}
	}
	return count
}

// RemoveMatchingSection removes all items from the queue that are runs with the specified section
func (q *SrQueue) RemoveMatchingSection(sec Section) {
	checkAndRemove := func(i int) {
		if q.items[i] != nil && q.items[i].Sec == sec {
			q.items[i] = nil
		}
	}
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		checkAndRemove(i)
	}
	checkAndRemove(q.tail)
}

// SRState is the state of the SectionRunner. All accesses synchronized over Mu
type SRState struct {
	Queue   SrQueue
	Current *SectionRun
	Mu      sync.Mutex
}

func newSRState() SRState {
	return SRState{
		newSRQueue(10), nil, sync.Mutex{},
	}
}

// SectionRunner runs a queue of sections
type SectionRunner struct {
	run    chan SectionRun
	cancel chan Section
	quit   chan struct{}
	state  SRState
	log    *logrus.Entry
}

// NewSectionRunner creates a new SectionRunner without starting it
func NewSectionRunner() *SectionRunner {
	return &SectionRunner{
		make(chan SectionRun, 2), make(chan Section, 2), make(chan struct{}),
		newSRState(),
		Logger.WithField("module", "SectionRunner"),
	}
}

func (r *SectionRunner) start(wait *sync.WaitGroup) {
	state := &r.state
	var (
		delay <-chan time.Time
	)
	runItem := func() {
		if state.Current == nil {
			return
		}
		r.log.WithField("state", state).Info("running section")
		state.Current.Sec.SetState(true)
		delay = time.After(state.Current.Duration)
	}
	finishRun := func() {
		state.Current.Sec.SetState(false)
		delay = nil
		if state.Current.Done != nil {
			state.Current.Done <- state.Queue.Len()
		}
		r.log.WithField("state", state).Info("finished running section")
		state.Current = state.Queue.Pop()
	}
	if wait != nil {
		defer wait.Done()
	}
	for {
		select {
		case <-r.quit:
			r.log.Debug("quiting section runner")
			return
		case item := <-r.run:
			state.Mu.Lock()
			if state.Current == nil {
				state.Current = &item
				runItem()
			} else {
				state.Queue.Push(&item)
				r.log.WithField("state", state).Debug("queued section run")
			}
			state.Mu.Unlock()
		case cancelSec := <-r.cancel:
			state.Mu.Lock()
			state.Queue.RemoveMatchingSection(cancelSec)
			if state.Current != nil && state.Current.Sec == cancelSec {
				finishRun()
				runItem()
			}
			r.log.WithFields(logrus.Fields{
				"state": state, "sec": cancelSec.Name(),
			}).Debug("cancelled section run")
			state.Mu.Unlock()
		case <-delay:
			state.Mu.Lock()
			finishRun()
			runItem()
			state.Mu.Unlock()
		}
	}
}

// Start starts the background goroutine of a SectionRunner
func (r *SectionRunner) Start(wait *sync.WaitGroup) {
	if wait != nil {
		wait.Add(1)
	}
	go r.start(wait)
}

// Quit tells the background goroutine to stop
func (r *SectionRunner) Quit() {
	r.quit <- struct{}{}
}

// QueueSectionRun queues the specified Section to run for dur
func (r *SectionRunner) QueueSectionRun(sec Section, dur time.Duration) {
	r.run <- SectionRun{sec, dur, nil, nil}
}

// RunSectionAsync runs the section and returns a chan which recieves when the section is finished running
func (r *SectionRunner) RunSectionAsync(sec Section, dur time.Duration) <-chan int {
	done := make(chan int, 1)
	r.run <- SectionRun{sec, dur, done, nil}
	return done
}

// RunSection runs the section and returns when the section is finished running
func (r *SectionRunner) RunSection(sec Section, dur time.Duration) {
	<-r.RunSectionAsync(sec, dur)
}

// CancelSection cancels all runs for the specified Section
func (r *SectionRunner) CancelSection(sec Section) {
	r.cancel <- sec
}
