package service

import (
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
)

// Service 是各业务 service 的公共依赖。
// 涉及 GORM + Casbin 同时变更的写请用 repo 层的 *Atomic 方法（真原子）。
type Service struct {
	logger *log.Logger
	jwt    *jwt.JWT
}

func NewService(
	logger *log.Logger,
	jwtUtil *jwt.JWT,
) *Service {
	return &Service{
		logger: logger,
		jwt:    jwtUtil,
	}
}
