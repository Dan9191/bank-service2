package service

import (
	"github.com/Dan9191/bank-service/internal/repository"
	"github.com/sirupsen/logrus"
)

// Service handles business logic
type Service struct {
	repo *repository.Repository
	log  *logrus.Logger
}

// NewService initializes a new service
func NewService(repo *repository.Repository, log *logrus.Logger) *Service {
	return &Service{repo: repo, log: log}
}
