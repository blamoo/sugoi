package main

import (
	"fmt"
	"os"
	"path"
)

type ReindexJob struct {
	Running       bool
	RequestCancel bool `json:"-"`
	Processed     int
	Total         int
	Error         int
	Ok            int
	Log           []string
}

func (job *ReindexJob) Start() error {
	const BATCH_LIMIT = 500
	if job.Running {
		return fmt.Errorf("Reindex already started")
	}

	job.Running = true
	job.RequestCancel = false
	job.Processed = 0
	job.Total = len(filePointers.List)
	job.Error = 0
	job.Ok = 0
	job.Log = nil

	go func() {
		var err error
		job.Log = append(job.Log, "Closing old index")
		bleveIndex.Close()

		job.Log = append(job.Log, "Removing old index")
		os.RemoveAll(path.Join(config.DatabaseDir, "sugoi.bleve"))

		job.Log = append(job.Log, "Creating new index")
		InitializeBleve()

		batch := bleveIndex.NewBatch()

		i := 0
		for _, file := range filePointers.List {

			if job.RequestCancel {
				job.Log = append(job.Log, "Cancelled")
				break
			}

			thing, _ := NewThingFromHash(file.Hash)
			thing.ListFilesRaw()

			err = file.ReindexIntoBatch(batch)
			if err != nil {
				job.Error++
				job.Processed++
				job.Log = append(job.Log, fmt.Sprintf("%s: %s", file.Key, err.Error()))
			} else {
				i++
				job.Ok++
				job.Processed++
				// this.Log = append(this.Log, fmt.Sprintf("%s: %s", file.Key, "OK"))
			}

			if i >= BATCH_LIMIT {
				job.Log = append(job.Log, fmt.Sprintf("Processing batch of %d files", BATCH_LIMIT))
				i = 0
				err = bleveIndex.Batch(batch)
				if err != nil {
					job.Log = append(job.Log, fmt.Sprintf("Error: %s", err.Error()))
				} else {
					if job.Total != 0 {
						job.Log = append(job.Log, fmt.Sprintf("%.1f%% done", (float64(job.Processed)/float64(job.Total))*100.0))
					}
				}
				batch = bleveIndex.NewBatch()
			}
		}

		if i > 0 {
			job.Log = append(job.Log, fmt.Sprintf("Processing final batch of %d files", i))
			err = bleveIndex.Batch(batch)
			if err != nil {
				job.Log = append(job.Log, fmt.Sprintf("Error: %s", err.Error()))
			} else {
				job.Log = append(job.Log, "100% done!")
			}
		}

		if config.Debug {
			v, _ := bleveIndex.Fields()
			fmt.Println(v)
		}

		job.Running = false
	}()

	return nil
}
