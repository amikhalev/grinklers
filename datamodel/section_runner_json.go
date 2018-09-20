package datamodel

import (
	"time"

	"git.amikhalev.com/amikhalev/grinklers/logic"
)

// SectionRunJSON is the JSON representation of a SectionRun
type SectionRunJSON struct {
	ID            int32      `json:"id"`
	Section       int        `json:"section"`
	TotalDuration float64    `json:"totalDuration"`
	Duration      float64    `json:"duration"`
	StartTime     *time.Time `json:"startTime"`
	PauseTime     *time.Time `json:"pauseTime"`
	UnpauseTime   *time.Time `json:"unpauseTime"`
}

// SectionRunToJSON returns an the JSON representation of this SectionRun, or err if there was some error.
// sections is a slice of the sections (in order to compute the section index)
func SectionRunToJSON(sr *logic.SectionRun) (j SectionRunJSON, err error) {
	j = SectionRunJSON{
		sr.RunID, sr.Sec.ID, sr.TotalDuration.Seconds(), sr.Duration.Seconds(),
		sr.StartTime, sr.PauseTime, sr.UnpauseTime,
	}
	return
}

// SRQueueToJSON returns the JSON representation of this SRQueue, or err if there was an error.
// See SectionRun#toJSON
func SRQueueToJSON(q *logic.SRQueue) (slice []SectionRunJSON, err error) {
	slice = nil
	var json SectionRunJSON
	q.ForEach(func(run *logic.SectionRun) bool {
		json, err = SectionRunToJSON(run)
		if err != nil {
			return false
		}
		slice = append(slice, json)
		return true
	})
	return
}

// SRStateJSON is the JSON representation of a SRState
type SRStateJSON struct {
	Queue   []SectionRunJSON `json:"queue"`
	Current *SectionRunJSON  `json:"current"`
	Paused  bool             `json:"paused"`
}

// SRStateToJSON returns the JSON representation of a SRState, or an error
func SRStateToJSON(s *logic.SRState) (json SRStateJSON, err error) {
	json = SRStateJSON{}
	json.Queue, err = SRQueueToJSON(&s.Queue)
	if err != nil {
		return
	}
	if s.Current != nil {
		var current SectionRunJSON
		current, err = SectionRunToJSON(s.Current)
		json.Current = &current
	} else {
		json.Current = nil
	}
	json.Paused = s.Paused
	return
}
