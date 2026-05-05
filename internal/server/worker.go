package server

import (
	"context"

	"github.com/shlewislee/typos/internal/printer"
)

func (s *Server) startWorker(ctx context.Context) {
	s.Logger.Debug("Starting worker")
	go func() {
		for {
			select {
			case <-ctx.Done():
				s.Logger.Debug("Worker stopped")
				return
			case job := <-s.jobs.Queue():
				s.Logger.Debug("Worker picked up job", "id", job.ID, "type", job.Type)
				s.processJob(job)
			}
		}
	}()
}

func (s *Server) processJob(job *Job) {
	if s.printerConn == nil {
		s.jobs.Update(job.ID, StatusFailed, "printer connection is uninitialized")
		s.Logger.Error("Job failed: printer connection is uninitialized", "id", job.ID)
		return
	}

	s.Logger.Debug("Processing job", "id", job.ID)
	s.jobs.Update(job.ID, StatusProcessing, "")

	defer func() {
		s.Logger.Debug("Cleaning up temporary files for job", "id", job.ID)
		job.Cleanup(s.Logger, s.tempDir)
	}()

	err := s.printerConn.Execute(func(p *printer.Printer) error {
		return job.Execute(p, &s.DefaultImageOptions, s.tempDir)
	})
	if err != nil {
		s.jobs.Update(job.ID, StatusFailed, err.Error())
		s.Logger.Error("Job failed", "id", job.ID, "error", err)
		return
	}

	s.jobs.Update(job.ID, StatusDone, "")
	s.Logger.Info("Job completed", "id", job.ID)
}
