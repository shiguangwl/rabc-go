package task

import (
	"nunu-layout-admin/internal/repository"
	"nunu-layout-admin/pkg/jwt"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/sid"
)

type Task struct {
	logger *log.Logger
	sid    *sid.Sid
	jwt    *jwt.JWT
	tm     repository.Transaction
}

func NewTask(
	tm repository.Transaction,
	logger *log.Logger,
	sid *sid.Sid,
) *Task {
	return &Task{
		logger: logger,
		sid:    sid,
		tm:     tm,
	}
}
