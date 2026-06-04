package services

import (
	"context"
	"errors"
	"time"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
)

type JobService struct {
	repo repositories.SyncJobRepository
}

const DefaultJobRetention = 30 * 24 * time.Hour

func NewJobService(repo repositories.SyncJobRepository) *JobService {
	return &JobService{repo: repo}
}

func (s *JobService) Start(ctx context.Context, panelID *int64, jobType string, retryCount int64) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("job service is not configured")
	}
	if !models.IsValidJobType(jobType) {
		return 0, errors.New("invalid job type")
	}
	job := &models.SyncJob{
		PanelID:    panelID,
		JobType:    jobType,
		Status:     models.SyncJobStatusRunning,
		RetryCount: retryCount,
		CreatedAt:  time.Now().UTC(),
	}
	job.StartedAt = &job.CreatedAt
	if err := s.repo.Create(ctx, job); err != nil {
		return 0, err
	}
	return job.ID, nil
}

func (s *JobService) Update(ctx context.Context, jobID int64, status, message string, finishedAt *time.Time) error {
	if s == nil || s.repo == nil {
		return errors.New("job service is not configured")
	}
	job, err := s.repo.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = status
	job.Message = message
	job.FinishedAt = finishedAt
	return s.repo.Update(ctx, job)
}

func (s *JobService) Get(ctx context.Context, jobID int64) (*models.SyncJob, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("job service is not configured")
	}
	return s.repo.FindByID(ctx, jobID)
}

func (s *JobService) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("job service is not configured")
	}
	if retention <= 0 {
		retention = DefaultJobRetention
	}
	return s.repo.DeleteCompletedBefore(ctx, time.Now().UTC().Add(-retention))
}
