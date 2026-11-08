package catalog

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const reindexBatchSize = 500

// ReindexJob rebuilds the whole search index in the background. Its state is
// read by the status page while the job writes it, so every access goes
// through the mutex; Status returns a copy.
type ReindexJob struct {
	catalog *Catalog

	mu            sync.Mutex
	status        ReindexStatus
	cancelRequest bool
}

// ReindexStatus is a snapshot of the job, as shown on /system.
type ReindexStatus struct {
	Running    bool
	FinishTime time.Time
	Processed  int
	Total      int
	Error      int
	Ok         int
	Log        []string
}

func (job *ReindexJob) Status() ReindexStatus {
	job.mu.Lock()
	defer job.mu.Unlock()

	s := job.status
	s.Log = append([]string(nil), job.status.Log...)
	return s
}

func (job *ReindexJob) Running() bool {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.status.Running
}

// Cancel asks a running job to stop after the current item. The index is then
// incomplete until the next full reindex.
func (job *ReindexJob) Cancel() {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.cancelRequest = true
}

// Wait blocks until the job is not running or the timeout passes, and reports
// whether it stopped.
func (job *ReindexJob) Wait(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for job.Running() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
	return true
}

func (job *ReindexJob) update(fn func(s *ReindexStatus)) {
	job.mu.Lock()
	defer job.mu.Unlock()
	fn(&job.status)
}

func (job *ReindexJob) log(format string, args ...any) {
	job.update(func(s *ReindexStatus) {
		s.Log = append(s.Log, fmt.Sprintf(format, args...))
	})
}

func (job *ReindexJob) cancelled() bool {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.cancelRequest
}

// Start empties the index and indexes every file of the library in batches,
// in a goroutine.
func (job *ReindexJob) Start() error {
	c := job.catalog
	lib := c.Library()

	job.mu.Lock()
	if job.status.Running {
		job.mu.Unlock()
		return fmt.Errorf("reindex already started")
	}
	job.status = ReindexStatus{Running: true, Total: len(lib.List)}
	job.cancelRequest = false
	job.mu.Unlock()

	go func() {
		defer func() {
			c.Autocomplete.MarkDirty()
			job.update(func(s *ReindexStatus) {
				s.FinishTime = time.Now()
				s.Running = false
			})
		}()

		job.log("Recreating the index")
		err := c.Index.Recreate()
		if err != nil {
			job.log("Error: %s", err)
			slog.Error("recreating the search index", "err", err)
			return
		}

		batch := c.Index.NewBatch()
		pending := 0
		cancelled := false

		percent := func() float64 {
			s := job.Status()
			return float64(s.Processed) / float64(s.Total) * 100
		}

		for _, file := range lib.List {
			if job.cancelled() {
				job.log("Cancelled")
				cancelled = true
				break
			}

			// warms the page-list cache for the reader
			if thing, err := c.Get(file.Hash); err == nil {
				thing.PageFiles()
			}

			err = batch.Index(file.Hash, c.document(file))
			if err != nil {
				job.update(func(s *ReindexStatus) {
					s.Error++
					s.Processed++
					s.Log = append(s.Log, fmt.Sprintf("%s: %s", file.Key, err))
				})
				continue
			}

			pending++
			job.update(func(s *ReindexStatus) {
				s.Ok++
				s.Processed++
			})

			if pending >= reindexBatchSize {
				job.log("(%.1f%%) Processing batch of %d files", percent(), pending)
				err = c.Index.Batch(batch)
				if err != nil {
					job.log("(%.1f%%) Error: %s", percent(), err)
				}
				batch = c.Index.NewBatch()
				pending = 0
			}
		}

		if pending > 0 {
			job.log("(%.1f%%) Processing final batch of %d files", percent(), pending)
			err = c.Index.Batch(batch)
			if err != nil {
				job.log("Error: %s", err)
				return
			}
		}
		if !cancelled {
			job.log("Reindex finished")
		}

		if fields, err := c.Index.Fields(); err == nil {
			slog.Debug("indexed fields", "fields", fields)
		}
	}()

	return nil
}
