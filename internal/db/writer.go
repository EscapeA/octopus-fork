package db

import (
	"context"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/utils/log"
)

type WriteJob struct {
	Name string
	Fn   func(ctx context.Context) error
}

var (
	writeQueue chan WriteJob
	writerOnce sync.Once
)

func StartSerialWriter(ctx context.Context) {
	writerOnce.Do(func() {
		writeQueue = make(chan WriteJob, 32)
		go func() {
			for {
				select {
				case job := <-writeQueue:
					if err := job.Fn(ctx); err != nil {
						log.Warnf("serial DB write job %q failed: %v", job.Name, err)
					}
				case <-ctx.Done():
					for {
						select {
						case job := <-writeQueue:
							if err := job.Fn(context.Background()); err != nil {
								log.Warnf("shutdown: serial DB write job %q failed: %v", job.Name, err)
							}
						default:
							return
						}
					}
				}
			}
		}()
	})
}

// EnqueueWrite submits a job to the serial writer. If the queue is full,
// it waits up to 5 seconds; if still full, it executes synchronously as
// a fallback to avoid data loss.
func EnqueueWrite(job WriteJob) {
	if writeQueue == nil {
		job.Fn(context.Background())
		return
	}
	select {
	case writeQueue <- job:
	case <-time.After(5 * time.Second):
		log.Warnf("serial DB write queue full (timeout), executing job %q synchronously", job.Name)
		if err := job.Fn(context.Background()); err != nil {
			log.Warnf("sync fallback DB write job %q failed: %v", job.Name, err)
		}
	}
}
