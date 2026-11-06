package main

import (
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/aliyun"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/credential"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/handler"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/sirupsen/logrus"
)

func main() {
	defer func() {
		if p := recover(); p != nil {
			logrus.Errorf("panic: %v", debug.Stack())
		}
	}()

	config.Parse()
	logrus.Infof("ai-dashboard starting, credential mode: %s, auth mode: %s", config.Get().CredentialMode, config.Get().AuthMode)
	if config.Get().AuthMode == auth.ModeCustom {
		logrus.Warn(`auth mode "custom" trusts identity headers from the upstream gateway; ` +
			"ensure the gateway authenticates every request and strips client-supplied identity headers")
	}

	// K8s client
	kubeClient, err := k8s.NewClient()
	if err != nil {
		logrus.Fatalf("failed to init k8s client: %v", err)
	}

	// Aliyun credentials and the IMS client are only required when
	// authenticating against Alibaba Cloud RAM (auth mode "aliyun").
	var imsClient *aliyun.IMSClient
	if config.Get().AuthMode == auth.ModeAliyun {
		// Credential provider
		provider, err := credential.NewProvider(config.Get())
		if err != nil {
			logrus.Fatalf("failed to init credential provider: %v", err)
		}
		credManager := credential.NewManager(provider)

		// IMS client (for RAM user listing + web app management)
		imsClient, err = aliyun.NewIMSClient(credManager, config.Get())
		if err != nil {
			logrus.Fatalf("failed to init IMS client: %v", err)
		}
	} else {
		logrus.Infof("auth mode %q: Aliyun credential init skipped", config.Get().AuthMode)
	}

	// OAuth2 SSO auth manager
	authManager, err := auth.NewManager(config.Get(), kubeClient, imsClient)
	if err != nil {
		logrus.Fatalf("failed to init auth manager: %v", err)
	}

	// Services
	ramService := service.NewRamService(imsClient)
	userService := service.NewUserService(kubeClient)
	quotaService := service.NewQuotaService(kubeClient)
	userGroupService := service.NewUserGroupService(kubeClient)
	datasetService := service.NewDatasetService(kubeClient)
	costService := service.NewCostService(kubeClient)
	modelAdminService := service.NewModelAdminService(kubeClient)

	// Startup initialization
	initService := service.NewInitService(kubeClient)
	if err := initService.Initialize(); err != nil {
		logrus.Errorf("startup initialization failed: %v", err)
	}

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logrus.Info("received exit signal, cleaning up...")
		authManager.Cleanup()
		time.Sleep(time.Second)
		os.Exit(0)
	}()

	// Router
	r := handler.NewRouter(config.Get(), kubeClient, authManager, userService, quotaService, userGroupService, datasetService, ramService, costService, modelAdminService)

	addr := config.Get().ListenAddr
	logrus.Infof("ai-dashboard listening on %s", addr)
	if err := r.Run(addr); err != nil {
		logrus.Fatalf("server failed: %v", err)
	}
}
