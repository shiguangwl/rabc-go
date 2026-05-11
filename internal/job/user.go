package job

import (
	"context"
	"nunu-layout-admin/internal/repository"
)

type UserJob interface {
	KafkaConsumer(ctx context.Context) error
}

func NewUserJob(
	job *Job,
	userRepo repository.UserRepository,
) UserJob {
	return &userJob{
		userRepo: userRepo,
		Job:      job,
	}
}

type userJob struct {
	userRepo repository.UserRepository
	*Job
}

// KafkaConsumer 是 Kafka 消费骨架，接入消息源后再注入 sarama/segmentio client。
// 当前空实现仅满足 wire 装配，无副作用。
func (t userJob) KafkaConsumer(ctx context.Context) error {
	return nil
}
