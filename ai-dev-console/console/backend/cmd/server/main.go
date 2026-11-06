package main

import (
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/aliyun"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/credential"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/handler"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
	"github.com/sirupsen/logrus"
)

func main() {
	defer func() {
		if p := recover(); p != nil {
			logrus.Errorf("panic: %v\n%s", p, debug.Stack())
		}
	}()

	config.Parse()
	logrus.Infof("ai-dev-console starting, credential mode: %s, auth mode: %s", config.Get().CredentialMode, config.Get().AuthMode)
	if config.Get().AuthMode == auth.ModeCustom {
		logrus.Warn(`auth mode "custom" trusts identity headers from the upstream gateway; ` +
			"ensure the gateway authenticates every request and strips client-supplied identity headers")
	}

	// K8s admin client
	kubeClient, err := k8s.NewClient()
	if err != nil {
		logrus.Fatalf("failed to init k8s client: %v", err)
	}

	// Aliyun credentials and the IMS client are only required when
	// authenticating against Alibaba Cloud RAM (auth mode "aliyun").
	var imsClient *ims.Client
	if config.Get().AuthMode == auth.ModeAliyun {
		// Credential provider
		provider, err := credential.NewProvider(config.Get())
		if err != nil {
			logrus.Fatalf("failed to init credential provider: %v", err)
		}
		credManager := credential.NewManager(provider)

		// IMS client
		imsClient, err = aliyun.NewIMSClient(credManager, config.Get())
		if err != nil {
			logrus.Fatalf("failed to init IMS client: %v", err)
		}
	} else {
		logrus.Infof("auth mode %q: Aliyun credential init skipped", config.Get().AuthMode)
	}

	// Auth manager (OAuth2 SSO)
	authManager, err := auth.NewManager(config.Get(), kubeClient, imsClient)
	if err != nil {
		logrus.Fatalf("failed to init auth manager: %v", err)
	}

	// Multi-tenant registry
	tenants := k8s.NewTenantRegistry(kubeClient)

	// Services
	notebookService := service.NewNotebookService(kubeClient, tenants)
	trainingService := service.NewTrainingService(kubeClient, tenants)
	servingService := service.NewServingService(kubeClient, tenants)
	datasetService := service.NewDatasetService(kubeClient, tenants)
	modelService := service.NewModelService(kubeClient, tenants)
	metricsService := service.NewMetricsService(kubeClient, tenants)
	experimentService := service.NewExperimentService(kubeClient, tenants)

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
	r := handler.NewRouter(config.Get(), authManager, notebookService, trainingService, servingService, datasetService, modelService, metricsService, experimentService)

	addr := config.Get().ListenAddr
	logrus.Infof("ai-dev-console listening on %s", addr)
	if err := r.Run(addr); err != nil {
		logrus.Fatalf("server failed: %v", err)
	}
}
