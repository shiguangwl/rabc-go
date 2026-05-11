package task

import (
	"context"
	"rabc-go/internal/repository"
)

type UserTask interface {
	CheckUser(ctx context.Context) error
}

func NewUserTask(
	task *Task,
	userRepo repository.UserRepository,
) UserTask {
	return &userTask{
		userRepo: userRepo,
		Task:     task,
	}
}

type userTask struct {
	userRepo repository.UserRepository
	*Task
}

// CheckUser 是定时检查骨架，仅打印日志验证调度链路；接入实际巡检逻辑替换它。
func (t userTask) CheckUser(ctx context.Context) error {
	t.logger.Info("CheckUser")
	return nil
}
