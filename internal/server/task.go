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
		t.log.Error("TaskServer Panic", zap.String("job", jobName), zap.Any("recover", recoverData))
	})

	t.scheduler = gocron.NewScheduler(time.UTC)

	_, err := t.scheduler.CronWithSeconds("0/3 * * * * *").Do(func() {
		err := t.userTask.CheckUser(ctx)
		if err != nil {
			t.log.Error("CheckUser error", zap.Error(err))
		}
	})
	if err != nil {
		t.log.Error("CheckUser error", zap.Error(err))
	}

	t.scheduler.StartBlocking()
	return nil
}
func (t *TaskServer) Stop(ctx context.Context) error {
	t.scheduler.Stop()
	t.log.Info("TaskServer stop...")
	return nil
}
