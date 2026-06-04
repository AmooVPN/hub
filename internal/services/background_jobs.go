package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
)

type BackgroundJobFunc func(context.Context) error

type backgroundJobTask struct {
	job  *models.SyncJob
	work BackgroundJobFunc
}

type BackgroundJobRunner struct {
	repo    repositories.SyncJobRepository
	workers int
	queue   chan backgroundJobTask
	once    sync.Once
	wg      sync.WaitGroup
}

func NewBackgroundJobRunner(repo repositories.SyncJobRepository, workers int) *BackgroundJobRunner {
	if workers <= 0 {
		workers = 1
	}
	return &BackgroundJobRunner{repo: repo, workers: workers, queue: make(chan backgroundJobTask, workers*4)}
}

func (r *BackgroundJobRunner) Start(ctx context.Context) {
	if r == nil || r.repo == nil {
		return
	}
	r.once.Do(func() {
		for i := 0; i < r.workers; i++ {
			r.wg.Add(1)
			go r.worker(ctx)
		}
	})
}

func (r *BackgroundJobRunner) Submit(ctx context.Context, jobType string, panelID *int64, retryCount int64, work BackgroundJobFunc) (int64, error) {
	if r == nil || r.repo == nil {
		return 0, errors.New("background job runner is not configured")
	}
	if !models.IsValidJobType(jobType) {
		return 0, fmt.Errorf("invalid job type %q", jobType)
	}
	if work == nil {
		return 0, errors.New("background job work is nil")
	}
	job := &models.SyncJob{PanelID: panelID, JobType: jobType, Status: models.SyncJobStatusQueued, RetryCount: retryCount, CreatedAt: time.Now().UTC()}
	if err := r.repo.Create(ctx, job); err != nil {
		return 0, err
	}
	r.queue <- backgroundJobTask{job: job, work: work}
	return job.ID, nil
}

func (r *BackgroundJobRunner) worker(root context.Context) {
	defer r.wg.Done()
	for task := range r.queue {
		ctx := root
		if ctx == nil {
			ctx = context.Background()
		}
		r.runTask(ctx, task)
	}
}

func (r *BackgroundJobRunner) runTask(ctx context.Context, task backgroundJobTask) {
	if task.job == nil {
		return
	}
	now := time.Now().UTC()
	task.job.Status = models.SyncJobStatusRunning
	task.job.StartedAt = &now
	task.job.Message = ""
	if err := r.repo.Update(ctx, task.job); err != nil {
		return
	}
	status := models.SyncJobStatusSuccess
	message := ""
	if err := task.work(ctx); err != nil {
		status = models.SyncJobStatusFailed
		message = err.Error()
	}
	finished := time.Now().UTC()
	task.job.Status = status
	task.job.Message = message
	task.job.FinishedAt = &finished
	_ = r.repo.Update(ctx, task.job)
}

func (r *BackgroundJobRunner) Close() error {
	if r == nil {
		return nil
	}
	close(r.queue)
	r.wg.Wait()
	return nil
}

func (r *BackgroundJobRunner) String() string {
	if r == nil {
		return "background job runner is nil"
	}
	return fmt.Sprintf("background job runner workers=%d", r.workers)
}
