package service

import (
	"nunu-layout-admin/internal/repository"
	"nunu-layout-admin/pkg/jwt"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/sid"
)

type Service struct {
	logger *log.Logger
	sid    *sid.Sid
	jwt    *jwt.JWT
	tm     repository.Transaction
}

func NewService(
	tm repository.Transaction,
	logger *log.Logger,
	sid *sid.Sid,
	jwt *jwt.JWT,
) *Service {
	return &Service{
		logger: logger,
		sid:    sid,
		jwt:    jwt,
		tm:     tm,
	}
}
