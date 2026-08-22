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

type Job struct {
	ID     string
	CN     string
	ctx    context.Context
	cancel context.CancelFunc

	mu         sync.Mutex
	state      State
	logFile    *os.File
	subs       map[chan protocol.StreamEvent]struct{}
	terminal   *protocol.StreamEvent
	finishedAt time.Time
}

func (j *Job) logPath() string {
	return filepath.Join(serverConfig.ServerConfigPath, "jobs", j.ID+".jsonl")
}

func (j *Job) Write(event protocol.StreamEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.state != StateRunning {
		return // too late, racc finished his job
	}

	if err := json.NewEncoder(j.logFile).Encode(event); err != nil {
		log.Printf("racc's job %s: couldn't append to log: %v", j.ID, err)
	}

	for sub := range j.subs {
		select {
		case sub <- event:
		default: // slow client, drop rather than stall the job
		}
	}
}

func (j *Job) WriteProgress(message string) {
	j.Write(protocol.StreamEvent{Type: "progress", Message: message})
}

func (j *Job) WriteOutput(message string) {
	j.Write(protocol.StreamEvent{Type: "output", Message: message})
}

type JobWriter struct {
	Job *Job
}

func (w *JobWriter) Write(p []byte) (int, error) {
	w.Job.WriteOutput(string(p))
	return len(p), nil
}

func (j *Job) Finish(state State, terminal protocol.StreamEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.state != StateRunning {
		return // gg
	}
	j.state = state
	j.finishedAt = time.Now()

	// keep the terminal event, the log is gone after this
	j.terminal = &terminal

	if err := json.NewEncoder(j.logFile).Encode(terminal); err != nil {
		log.Printf("racc's job %s: couldn't append terminal event to log: %v", j.ID, err)
	}
	j.logFile.Close()

	// delete the log, no point keeping it once racc's job is done
	if err := os.Remove(j.logPath()); err != nil && !os.IsNotExist(err) {
		log.Printf("racc's job %s: couldn't remove log file: %v", j.ID, err)
	}

	for sub := range j.subs {
		select {
		case sub <- terminal:
		default:
		}
		close(sub) // tell stream handlers it's over
	}

	j.cancel()
}

// registers a subscriber and returns the log size, so callers can replay
// whats already logged and only get new events live (no duplication)
func (j *Job) Subscribe() (sub chan protocol.StreamEvent, endOffset int64, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.state != StateRunning {
		// job's over and the log is gone, replay handles the terminal event
		sub = make(chan protocol.StreamEvent, 16)
		close(sub)
		return sub, 0, nil
	}

	info, err := j.logFile.Stat()
	if err != nil {
		return nil, 0, err
	}

	sub = make(chan protocol.StreamEvent, 16)
	j.subs[sub] = struct{}{}
	return sub, info.Size(), nil
}

func (j *Job) Replay(emit func(protocol.StreamEvent) error, endOffset int64) error {
	file, err := os.Open(j.logPath())
	if err != nil {
		// log's already gone, replay the terminal event we kept
		j.mu.Lock()
		terminal := j.terminal
		j.mu.Unlock()
		if terminal != nil {
			return emit(*terminal)
		}
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

func (j *Job) Unsubscribe(sub chan protocol.StreamEvent) {
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
		state:   StateRunning,
		logFile: logFile,
		subs:    make(map[chan protocol.StreamEvent]struct{}),
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
				job.Finish(StateError, protocol.StreamEvent{Type: "error", Message: fmt.Sprintf("racc's job panicked: %v", recovered)})
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
