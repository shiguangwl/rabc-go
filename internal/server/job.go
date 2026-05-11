package server

import (
	"context"
	"rabc-go/internal/job"
	"rabc-go/pkg/log"
)

type JobServer struct {
	log     *log.Logger
	userJob job.UserJob
}

func NewJobServer(
	log *log.Logger,
	userJob job.UserJob,
) *JobServer {
	return &JobServer{
		log:     log,
		userJob: userJob,
	}
}

func (j *JobServer) Start(ctx context.Context) error {
	if err := j.userJob.KafkaConsumer(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}
func (j *JobServer) Stop(ctx context.Context) error {
	return nil
}
