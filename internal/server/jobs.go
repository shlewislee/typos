package server

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/shlewislee/typos/internal/printer"
	"github.com/shlewislee/typos/internal/typst"
)

var ErrQueueFull = errors.New("job channel is full")

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusDone       JobStatus = "done"
	StatusFailed     JobStatus = "failed"
)

type JobType string

const (
	JobTypeTemplate JobType = "template"
	JobTypeFile     JobType = "file"
	JobTypeImage    JobType = "image"
)

type Job struct {
	ID     string    `json:"id"`
	Status JobStatus `json:"status"`
	Type   JobType   `json:"type"`
	Error  string    `json:"error,omitempty"`

	ImagePath    string                `json:"-"`
	ImageOptions *printer.ImageOptions `json:"-"`
	TemplatePath string                `json:"-"`
	FileContent  []byte                `json:"-"`
	Inputs       map[string]string     `json:"-"`
	FontPaths    []string              `json:"-"`
	JobDir       string                `json:"-"`
}

func (j *Job) Execute(p *printer.Printer, tempDir string) error {
	switch j.Type {
	case JobTypeImage:
		return j.executeImage(p)
	case JobTypeTemplate:
		return j.executeTemplate(p, tempDir)
	case JobTypeFile:
		return j.executeFile(p, tempDir)
	default:
		return errors.New("unknown job type")
	}
}

func (j *Job) Cleanup(logger *slog.Logger, tempDir string) {
	if j.JobDir == "" {
		return
	}
	logger.Debug("Removing job directory", "path", j.JobDir)
	if err := os.RemoveAll(filepath.Join(tempDir, j.JobDir)); err != nil {
		logger.Error("failed to remove job directory", "path", j.JobDir, "error", err)
	}
}

func (j *Job) executeImage(p *printer.Printer) error {
	p.Logger.Debug("Printing image", "path", j.ImagePath)
	return p.PrintImage(j.ImagePath, j.ImageOptions)
}

func (j *Job) executeTemplate(p *printer.Printer, tempDir string) error {
	content, err := os.ReadFile(j.TemplatePath)
	if err != nil {
		return err
	}
	return j.executeTypst(content, filepath.Base(j.TemplatePath), "printing template", p, tempDir)
}

func (j *Job) executeFile(p *printer.Printer, tempDir string) error {
	return j.executeTypst(j.FileContent, "main.typ", "printing raw file", p, tempDir)
}

func (j *Job) executeTypst(content []byte, filename, logMsg string, p *printer.Printer, tempDir string) error {
	jobDir := filepath.Join(tempDir, j.JobDir)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return err
	}

	fullPath := filepath.Join(jobDir, filename)
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		return err
	}

	printOpts := &printer.TypstOptions{
		ImageOptions: j.ImageOptions,
		RenderTypstOptions: &typst.CompileOptions{
			Input:     j.Inputs,
			DPI:       p.DPI,
			Root:      jobDir,
			FontPaths: j.FontPaths,
		},
	}
	p.Logger.Debug(logMsg, "path", fullPath)
	return p.PrintTypst(fullPath, printOpts)
}

func generateID() string {
	const idCharList = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	id, _ := gonanoid.Generate(idCharList, 6)

	return id
}

type Jobs struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	order   []string
	maxJobs int
	queue   chan *Job
}

func NewJobs(maxJobs int) *Jobs {
	return &Jobs{
		jobs:    make(map[string]*Job),
		order:   make([]string, 0, maxJobs),
		maxJobs: maxJobs,
		queue:   make(chan *Job, 100),
	}
}

func (j *Jobs) Enqueue(job *Job) (string, error) {
	if job.ID == "" {
		job.ID = generateID()
	}
	job.Status = StatusPending

	j.mu.Lock()
	defer j.mu.Unlock()

	select {
	case j.queue <- job:
		if len(j.jobs) >= j.maxJobs {
			oldestID := j.order[0]
			delete(j.jobs, oldestID)
			j.order = j.order[1:]
		}
		j.jobs[job.ID] = job
		j.order = append(j.order, job.ID)
		return job.ID, nil
	default:
		return "", ErrQueueFull
	}
}

func (j *Jobs) Update(id string, status JobStatus, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	job, ok := j.jobs[id]
	if !ok {
		return
	}
	job.Status = status
	if errMsg != "" {
		job.Error = errMsg
	}

	if status == StatusDone || status == StatusFailed {
		job.FileContent = nil
		job.Inputs = nil
		job.ImageOptions = nil
		job.FontPaths = nil
	}
}

func (j *Jobs) Get(id string) (Job, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	job, ok := j.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *job, true
}

func (j *Jobs) Queue() <-chan *Job {
	return j.queue
}
