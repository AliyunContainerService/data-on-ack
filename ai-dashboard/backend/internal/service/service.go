package service

import (
	"errors"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/aliyun"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
)

type RamService struct {
	imsClient *aliyun.IMSClient
}

func NewRamService(imsClient *aliyun.IMSClient) *RamService {
	return &RamService{imsClient: imsClient}
}

func (s *RamService) ListRamUsers() ([]aliyun.RamUser, error) {
	if s.imsClient == nil {
		return nil, errors.New("RAM user listing is unavailable: Aliyun credentials are not configured in the current auth mode")
	}
	return s.imsClient.ListUsers()
}

type InitService struct {
	kubeClient *k8s.Client
}

func NewInitService(kubeClient *k8s.Client) *InitService {
	return &InitService{kubeClient: kubeClient}
}
