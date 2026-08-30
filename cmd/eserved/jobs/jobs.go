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
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"git.fedesito.me/fedes1to/eserve/cmd/eserved/serverConfig"
	"git.fedesito.me/fedes1to/eserve/internal/protocol"
)

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateDone      State = "done"
	StateError     State = "error"
	StateCancelled State = "cancelled"
)

const (
	jobRetention    = time.Hour
	janitorInterval = 10 * time.Minute
	maxLogLine      = 1 << 20
	// how many jobs can wait for a flavor before we say no
	queueBuffer = 8
)

func isTerminal(state State) bool {
	return state == StateDone || state == StateError || state == StateCancelled
}

type Job struct {
	ID     string
	CN     string
	Flavor string // "" = never queued, runs right away
	Kind   string // "provision" or "build"

	ctx    context.Context
	cancel context.CancelFunc
	work   func(ctx context.Context, job *Job)

	mu         sync.Mutex
	state      State
	logFile    *os.File
	subs       map[chan protocol.StreamEvent]struct{}
	terminal   *protocol.StreamEvent
	startedAt  time.Time
	finishedAt time.Time
}

func (j *Job) logPath() string {
	return filepath.Join(serverConfig.ServerConfigPath, "jobs", j.ID+".jsonl")
}

func (j *Job) Write(event protocol.StreamEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if isTerminal(j.state) {
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

	if isTerminal(j.state) {
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

// only the flavor's queue worker calls this, and only for jobs that waited
func (j *Job) StartRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.state != StateQueued {
		return
	}
	j.state = StateRunning
}

func (j *Job) Info() protocol.JobInfo {
	j.mu.Lock()
	defer j.mu.Unlock()

	info := protocol.JobInfo{
		ID:         j.ID,
		CN:         j.CN,
		Flavor:     j.Flavor,
		Kind:       j.Kind,
		State:      string(j.state),
		Terminal:   j.terminalMessage(),
		StartedAt:  j.startedAt.Format(time.RFC3339),
		FinishedAt: j.finishedAt.Format(time.RFC3339),
	}
	return info
}

func (j *Job) terminalMessage() string {
	if j.terminal == nil {
		return ""
	}
	return j.terminal.Message
}

// returns the log size, so callers replay the past and get the rest live
func (j *Job) Subscribe() (sub chan protocol.StreamEvent, endOffset int64, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if isTerminal(j.state) {
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
	return isTerminal(j.State())
}

func (j *Job) Cancel() { j.cancel() }

// streams the job as SSE until it ends or the client goes away; cancel may be nil
func (j *Job) Stream(w http.ResponseWriter, cancel <-chan struct{}) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer can't flush")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sub, endOffset, err := j.Subscribe()
	if err != nil {
		return err
	}
	defer j.Unsubscribe(sub)

	if err := j.Replay(func(event protocol.StreamEvent) error {
		return writeStreamEvent(w, flusher, event)
	}, endOffset); err != nil {
		return err
	}
	if j.IsFinished() {
		return nil
	}

	for {
		select {
		case <-cancel:
			return nil
		case event, ok := <-sub:
			if !ok {
				return nil // the job's over
			}
			if err := writeStreamEvent(w, flusher, event); err != nil {
				return err // the client went away
			}
		}
	}
}

func writeStreamEvent(w http.ResponseWriter, flusher http.Flusher, event protocol.StreamEvent) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, strings.ReplaceAll(event.Message, "\n", "\ndata: "))
	flusher.Flush()
	return err
}

type JobRegistry struct {
	mu     sync.Mutex
	jobs   map[string]*Job
	queues map[string]chan *Job
}

var Registry = &JobRegistry{
	jobs:   make(map[string]*Job),
	queues: make(map[string]chan *Job),
}

var startJanitor sync.Once

// with a flavor, the job waits in that flavor's queue (one at a time); without one it runs now
func (r *JobRegistry) Start(cn, flavor, kind string, work func(ctx context.Context, job *Job)) (job *Job, err error) {
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
		ID:        id,
		CN:        cn,
		Flavor:    flavor,
		Kind:      kind,
		ctx:       ctx,
		cancel:    cancel,
		work:      work,
		logFile:   logFile,
		subs:      make(map[chan protocol.StreamEvent]struct{}),
		startedAt: time.Now(),
	}

	r.mu.Lock()
	r.jobs[id] = job
	r.mu.Unlock()

	if flavor == "" {
		job.mu.Lock()
		job.state = StateRunning
		job.mu.Unlock()
		go r.runWork(job)
		return job, nil
	}

	queue := r.ensureFlavorQueue(flavor)
	job.mu.Lock()
	job.state = StateQueued
	job.mu.Unlock()
	job.WriteProgress("waiting in the " + flavor + " queue")

	select {
	case queue <- job:
	default:
		job.Finish(StateError, protocol.StreamEvent{Type: "error", Message: "the " + flavor + " queue is full, try again in a bit"})
		return job, fmt.Errorf("the %s queue is full", flavor)
	}
	return job, nil
}

func (r *JobRegistry) ensureFlavorQueue(flavor string) chan *Job {
	r.mu.Lock()
	defer r.mu.Unlock()

	queue, ok := r.queues[flavor]
	if !ok {
		queue = make(chan *Job, queueBuffer)
		r.queues[flavor] = queue
		go r.serveQueue(flavor, queue)
	}
	return queue
}

func (r *JobRegistry) serveQueue(flavor string, queue chan *Job) {
	for job := range queue {
		if job.IsFinished() {
			continue // cancelled while waiting, nothing to run
		}
		job.StartRunning()
		job.WriteProgress("flavor " + flavor + " is free, starting")
		r.runWorkBlocking(job)
	}
}

// work may finish the job itself; Finish is idempotent
func (r *JobRegistry) finishWorkedJob(job *Job) {
	job.Finish(StateDone, protocol.StreamEvent{Type: "done", Message: job.Kind + " complete"})
}

func (r *JobRegistry) runWork(job *Job) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				job.Finish(StateError, protocol.StreamEvent{Type: "error", Message: fmt.Sprintf("racc's job panicked: %v", recovered)})
			}
		}()
		job.work(job.ctx, job)
		r.finishWorkedJob(job)
	}()
}

// runs work with panic recovery, blocking the caller
func (r *JobRegistry) runWorkBlocking(job *Job) {
	defer func() {
		if recovered := recover(); recovered != nil {
			job.Finish(StateError, protocol.StreamEvent{Type: "error", Message: fmt.Sprintf("racc's job panicked: %v", recovered)})
		}
	}()
	job.work(job.ctx, job)
	r.finishWorkedJob(job)
}

func (r *JobRegistry) Get(id string) (job *Job, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok = r.jobs[id]
	return
}

// a snapshot of every job the registry knows about, oldest first
func (r *JobRegistry) List() []protocol.JobInfo {
	r.mu.Lock()
	list := make([]protocol.JobInfo, 0, len(r.jobs))
	for _, job := range r.jobs {
		list = append(list, job.Info())
	}
	r.mu.Unlock()

	sort.Slice(list, func(i, j int) bool {
		if list[i].StartedAt != list[j].StartedAt {
			return list[i].StartedAt < list[j].StartedAt
		}
		return list[i].ID < list[j].ID
	})
	return list
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
