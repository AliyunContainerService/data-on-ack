package service

import (
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
	return s.imsClient.ListUsers()
}

type InitService struct {
	kubeClient *k8s.Client
}

func NewInitService(kubeClient *k8s.Client) *InitService {
	return &InitService{kubeClient: kubeClient}
}
