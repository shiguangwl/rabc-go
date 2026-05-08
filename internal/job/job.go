package job

import (
	"nunu-layout-admin/internal/repository"
	"nunu-layout-admin/pkg/jwt"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/sid"
)

type Job struct {
	logger *log.Logger
	sid    *sid.Sid
	jwt    *jwt.JWT
	tm     repository.Transaction
}

func NewJob(
	tm repository.Transaction,
	logger *log.Logger,
	sid *sid.Sid,
) *Job {
	return &Job{
		logger: logger,
		sid:    sid,
		tm:     tm,
	}
}
