package server

import (
	"context"
	"rabc-go/internal/task"
	"rabc-go/pkg/log"
	"time"

	"github.com/go-co-op/gocron"
	"go.uber.org/zap"
)

type TaskServer struct {
	log       *log.Logger
	scheduler *gocron.Scheduler
	userTask  task.UserTask
}

func NewTaskServer(
	log *log.Logger,
	userTask task.UserTask,
) *TaskServer {
	return &TaskServer{
		log:      log,
		userTask: userTask,
	}
}
func (t *TaskServer) Start(ctx context.Context) error {
	gocron.SetPanicHandler(func(jobName string, recoverData interface{}) {
		t.log.Error("任务发生 panic 并已恢复", zap.String("job_name", jobName), zap.Any("recover_data", recoverData))
	})

	t.scheduler = gocron.NewScheduler(time.UTC)

	_, err := t.scheduler.CronWithSeconds("0/3 * * * * *").Do(func() {
		err := t.userTask.CheckUser(ctx)
		if err != nil {
			t.log.Error("用户检查失败", zap.Error(err))
		}
	})
	if err != nil {
		t.log.Error("注册用户检查任务失败", zap.Error(err))
	}

	t.scheduler.StartBlocking()
	return nil
}
func (t *TaskServer) Stop(ctx context.Context) error {
	if t.scheduler == nil {
		return nil
	}
	t.scheduler.Stop()
	t.log.Info("任务服务已停止")
	return nil
}
