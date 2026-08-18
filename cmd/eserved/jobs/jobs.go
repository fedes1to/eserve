package jobs

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

type State string

const (
	StateRunning   State = "running"
	StateDone      State = "done"
	StateError     State = "error"
	StateCancelled State = "cancelled"
)

const (
	jobRetention    = time.Hour
	janitorInterval = 10 * time.Minute
	maxLogLine      = 1 << 20
)

type subscriber struct {
	mu     sync.Mutex
	queue  []protocol.StreamEvent
	notify chan struct{} // signals "drain me"
}

func (s *subscriber) push(event protocol.StreamEvent) {
	s.mu.Lock()
	s.queue = append(s.queue, event)
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *subscriber) Pop() (events []protocol.StreamEvent) {
	s.mu.Lock()
	events = s.queue
	s.queue = nil
	s.mu.Unlock()
	return
}

func (s *subscriber) Wait() <-chan struct{} { return s.notify }

type Job struct {
	ID     string
	CN     string
	ctx    context.Context
	cancel context.CancelFunc

	done chan struct{}

	mu         sync.Mutex
	state      State
	logFile    *os.File
	subs       map[*subscriber]struct{}
	finishedAt time.Time
}

func (j *Job) logPath() string {
	return filepath.Join(serverConfig.ServerConfigPath, "jobs", j.ID+".jsonl")
}

func (j *Job) Write(event protocol.StreamEvent) {
	j.mu.Lock()
	if j.state != StateRunning {
		j.mu.Unlock()
		return // too late, racc finished his job
	}

	// append to disk log, then fan out to live subscribers
	if err := json.NewEncoder(j.logFile).Encode(event); err != nil {
		log.Printf("racc job %s: couldn't append to log: %v", j.ID, err)
	}

	subscribers := make([]*subscriber, 0, len(j.subs))
	for sub := range j.subs {
		subscribers = append(subscribers, sub)
	}
	j.mu.Unlock()

	for _, sub := range subscribers {
		sub.push(event)
	}
}

func (j *Job) WriteProgress(message string) {
	j.Write(protocol.StreamEvent{Type: "progress", Message: message})
}

func (j *Job) WriteOutput(message string) {
	j.Write(protocol.StreamEvent{Type: "output", Message: message})
}

func (j *Job) Finish(state State, terminal protocol.StreamEvent) {
	j.mu.Lock()
	if j.state != StateRunning {
		j.mu.Unlock()
		return // gg
	}
	j.state = state
	j.finishedAt = time.Now()

	// terminal event goes to the log first, so late replays see it
	if err := json.NewEncoder(j.logFile).Encode(terminal); err != nil {
		log.Printf("racc job %s: couldn't append terminal event to log: %v", j.ID, err)
	}
	j.logFile.Close()

	subscribers := make([]*subscriber, 0, len(j.subs))
	for sub := range j.subs {
		subscribers = append(subscribers, sub)
	}
	j.mu.Unlock()

	for _, sub := range subscribers {
		sub.push(terminal)
	}

	j.cancel()
	close(j.done) // wake all stream handlers
}

// registers a subscriber and returns the log size, so callers can replay
// whats already logged and only get new events live (no duplication)
func (j *Job) Subscribe() (sub *subscriber, endOffset int64, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	info, err := j.logFile.Stat()
	if err != nil {
		return nil, 0, err
	}

	sub = &subscriber{notify: make(chan struct{}, 1)}
	j.subs[sub] = struct{}{}
	return sub, info.Size(), nil
}

func (j *Job) Replay(emit func(protocol.StreamEvent) error, endOffset int64) error {
	file, err := os.Open(j.logPath())
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(io.LimitReader(file, endOffset))
	scanner.Buffer(make([]byte, 64*1024), maxLogLine)
	for scanner.Scan() {
		var event protocol.StreamEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		if err := emit(event); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (j *Job) Unsubscribe(sub *subscriber) {
	j.mu.Lock()
	defer j.mu.Unlock()
	delete(j.subs, sub)
}

func (j *Job) State() (state State) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state
}

func (j *Job) IsFinished() bool {
	return j.State() != StateRunning
}

func (j *Job) Cancel() { j.cancel() }

func (j *Job) Done() <-chan struct{} { return j.done }

type JobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

var Registry = &JobRegistry{jobs: make(map[string]*Job)}

var startJanitor sync.Once

func (r *JobRegistry) Start(cn string, work func(ctx context.Context, job *Job)) (job *Job, err error) {
	id, err := newJobID()
	if err != nil {
		return nil, err
	}

	jobsDir := filepath.Join(serverConfig.ServerConfigPath, "jobs")
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return nil, err
	}

	logFile, err := os.Create(filepath.Join(jobsDir, id+".jsonl"))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	job = &Job{
		ID:      id,
		CN:      cn,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
		state:   StateRunning,
		logFile: logFile,
		subs:    make(map[*subscriber]struct{}),
	}

	r.mu.Lock()
	r.jobs[id] = job
	r.mu.Unlock()

	startJanitor.Do(func() {
		go func() {
			ticker := time.NewTicker(janitorInterval)
			defer ticker.Stop()
			for range ticker.C {
				r.cleanup()
			}
		}()
	})

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				job.Finish(StateError, protocol.StreamEvent{Type: "error", Message: fmt.Sprintf("racc job panicked: %v", recovered)})
			}
		}()
		work(ctx, job)
	}()

	return job, nil
}

func (r *JobRegistry) Get(id string) (job *Job, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok = r.jobs[id]
	return
}

func (r *JobRegistry) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, job := range r.jobs {
		if job.IsFinished() && time.Since(job.finishedAt) > jobRetention {
			job.logFile.Close()
			os.Remove(job.logPath())
			delete(r.jobs, id)
		}
	}
}

func newJobID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
