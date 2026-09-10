package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Snapshot struct {
	StartedAt          time.Time `json:"started_at"`
	LastPollAt         time.Time `json:"last_poll_at,omitempty"`
	LastSuccessAt      time.Time `json:"last_success_at,omitempty"`
	LastFlushAt        time.Time `json:"last_flush_at,omitempty"`
	Polls              int64     `json:"polls"`
	RecordsAccepted    int64     `json:"records_accepted"`
	RecordsRejected    int64     `json:"records_rejected"`
	RecordsBuffered    int64     `json:"records_buffered"`
	ConsecutiveFailure int64     `json:"consecutive_failure"`
	LastError          string    `json:"last_error,omitempty"`
	Healthy            bool      `json:"healthy"`
}

type State struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func NewState() *State {
	return &State{snapshot: Snapshot{StartedAt: time.Now().UTC(), Healthy: true}}
}

func (s *State) RecordPoll(accepted int, rejected int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Polls++
	s.snapshot.LastPollAt = time.Now().UTC()
	s.snapshot.RecordsAccepted += int64(accepted)
	s.snapshot.RecordsRejected += int64(rejected)
	if accepted > 0 {
		s.snapshot.LastSuccessAt = s.snapshot.LastPollAt
	}
	s.snapshot.ConsecutiveFailure = 0
	s.snapshot.LastError = ""
	s.snapshot.Healthy = true
}

func (s *State) RecordBuffer(count int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.RecordsBuffered += int64(count)
	s.snapshot.ConsecutiveFailure++
	if err != nil {
		s.snapshot.LastError = err.Error()
	}
	s.snapshot.Healthy = false
}

func (s *State) RecordFlush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.LastFlushAt = time.Now().UTC()
}

func (s *State) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		statusCode := http.StatusOK
		if !s.snapshot.Healthy {
			statusCode = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(s.snapshot)
	})
}
