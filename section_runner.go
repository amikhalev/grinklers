package grinklers

import (
	"fmt"
	"time"

	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
)

type SectionRunJSON struct {
	ID        int32      `json:"id"`
	Section   int        `json:"section"`
	Duration  float64    `json:"duration"`
	StartTime *time.Time `json:"startTime"`
}

type SRStateJSON struct {
	Queue   []SectionRunJSON `json:"queue"`
	Current *SectionRunJSON  `json:"current"`
}

// SectionRun is a single run of a section for a duration that is either queued, or currently running
type SectionRun struct {
	// RunID is a sequential unique identifier of SectionRuns
	RunID int32
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

func (sr *SectionRun) ToJSON(sections []Section) (j SectionRunJSON, err error) {
	j = SectionRunJSON{}
	secID := -1
	for i, sec := range sections {
		if sr.Sec == sec {
			secID = i
		}
	}
	if secID == -1 {
		err = fmt.Errorf("the section of this program does not exist in the sections array")
		return
	}
	j.ID = sr.RunID
	j.Section = secID
	j.Duration = sr.Duration.Seconds()
	j.StartTime = sr.StartTime
	return
}

// SRQueue is a queue for SectionRuns. It is implemented as a circular buffer that doubles in length when it fills up
type SRQueue struct {
	items []*SectionRun
	head  int
	tail  int
}

func newSRQueue(size int) SRQueue {
	return SRQueue{
		make([]*SectionRun, size),
		0, 0,
	}
}

// Format implements Formatter for SrQueue
func (q SRQueue) Format(f fmt.State, c rune) {
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

var _ fmt.Formatter = (*SRQueue)(nil)

func (q *SRQueue) ToJSON(sections []Section) (slice []SectionRunJSON, err error) {
	slice = nil
	var json SectionRunJSON
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			json, err = q.items[i].ToJSON(sections)
			if err != nil {
				return
			}
			slice = append(slice, json)
		}
	}
	return
}

// Push adds an item to the end of the SrQueue, expanding it if necessary
func (q *SRQueue) Push(item *SectionRun) {
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
func (q *SRQueue) Pop() *SectionRun {
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
func (q *SRQueue) Len() int {
	count := 0
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			count++
		}
	}
	return count
}

// RemoveMatchingSection removes all items from the queue that are runs with the specified section
func (q *SRQueue) RemoveMatchingSection(sec Section) {
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

// RemoveById removes the SectionRun with the specified id and returns it, or returns nil if none existed
func (q *SRQueue) RemoveById(id int32) *SectionRun {
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil && q.items[i].RunID == id {
			item := q.items[i]
			q.items[i] = nil
			return item
		}
	}
	return nil
}

// SRState is the state of the SectionRunner. All accesses synchronized over Mu
type SRState struct {
	Queue   SRQueue
	Current *SectionRun
	Mu      sync.Mutex
}

func newSRState() SRState {
	return SRState{
		newSRQueue(10), nil, sync.Mutex{},
	}
}

func (s *SRState) ToJSON(sections []Section) (json SRStateJSON, err error) {
	json = SRStateJSON{}
	json.Queue, err = s.Queue.ToJSON(sections)
	if err != nil {
		return
	}
	if s.Current != nil {
		var current SectionRunJSON
		current, err = s.Current.ToJSON(sections)
		json.Current = &current
	} else {
		json.Current = nil
	}
	return
}

// SectionRunner runs a queue of sections
type SectionRunner struct {
	run           chan SectionRun
	cancelSec     chan Section
	cancelID      chan int32
	quit          chan struct{}
	nextID        int32
	State         SRState
	OnUpdateState chan<- *SRState
	log           *logrus.Entry
}

// NewSectionRunner creates a new SectionRunner without starting it
func NewSectionRunner() *SectionRunner {
	return &SectionRunner{
		make(chan SectionRun, 2), make(chan Section, 2), make(chan int32, 2), make(chan struct{}),
		0, newSRState(), nil,
		Logger.WithField("module", "SectionRunner"),
	}
}

func (r *SectionRunner) start(wait *sync.WaitGroup) {
	r.stateUpdate()
	state := &r.State
	var (
		delay <-chan time.Time
	)
	runItem := func() {
		if state.Current == nil {
			return
		}
		r.log.WithField("state", state).Info("running section")
		state.Current.Sec.SetState(true)
		startTime := time.Now()
		state.Current.StartTime = &startTime
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
			r.startUpdate()
			if state.Current == nil {
				state.Current = &item
				runItem()
			} else {
				state.Queue.Push(&item)
				r.log.WithField("state", state).Debug("queued section run")
			}
			r.endUpdate()
		case sec := <-r.cancelSec:
			r.startUpdate()
			state.Queue.RemoveMatchingSection(sec)
			if state.Current != nil && state.Current.Sec == sec {
				finishRun()
				runItem()
			}
			r.log.WithFields(logrus.Fields{
				"state": state, "sec": sec.Name(),
			}).Debug("cancelled section runs with section")
			r.endUpdate()
		case id := <-r.cancelID:
			r.startUpdate()
			state.Queue.RemoveById(id)
			if state.Current != nil && state.Current.RunID == id {
				finishRun()
				runItem()
			}
			r.log.WithFields(logrus.Fields{
				"state": state, "id": id,
			}).Debug("cancelled section run by id")
			r.endUpdate()
		case <-delay:
			r.startUpdate()
			finishRun()
			runItem()
			r.endUpdate()
		}
	}
}

func (r *SectionRunner) stateUpdate() {
	if r.OnUpdateState != nil {
		r.OnUpdateState <- &r.State
	}
}

func (r *SectionRunner) startUpdate() {
	r.State.Mu.Lock()
}

func (r *SectionRunner) endUpdate() {
	r.State.Mu.Unlock()
	r.stateUpdate()
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

func (r *SectionRunner) getNextID() int32 {
	return atomic.AddInt32(&r.nextID, 1) - 1
}

// QueueSectionRun queues the specified Section to run for dur
func (r *SectionRunner) QueueSectionRun(sec Section, dur time.Duration) (id int32) {
	id = r.getNextID()
	r.run <- SectionRun{id, sec, dur, nil, nil}
	return
}

// RunSectionAsync runs the section and returns a chan which recieves when the section is finished running
func (r *SectionRunner) RunSectionAsync(sec Section, dur time.Duration) (id int32, done <-chan int) {
	id = r.getNextID()
	doneChan := make(chan int, 1)
	r.run <- SectionRun{r.getNextID(), sec, dur, doneChan, nil}
	done = doneChan
	return
}

// RunSection runs the section and returns when the section is finished running
func (r *SectionRunner) RunSection(sec Section, dur time.Duration) {
	_, done := r.RunSectionAsync(sec, dur)
	<-done
}

// CancelSection cancels all runs for the specified Section
func (r *SectionRunner) CancelSection(sec Section) {
	r.cancelSec <- sec
}

// CancelID cancels the section run with the specified id
func (r *SectionRunner) CancelID(id int32) {
	r.cancelID <- id
}
