package clientmgr

import (
	"context"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/types"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	setupLog               = ctrl.Log.WithName("setup")
	cmgr                   = &clientMgr{}
	_        ClientManager = cmgr
)

type clientMgr struct {
	config     *rest.Config
	scheme     *runtime.Scheme
	ctrlCache  cache.Cache
	ctrlClient client.Client
	kubeClient clientset.Interface
	arena      *arenaclient.ArenaClient
	ctx        context.Context
	cancel     context.CancelFunc
}

func Init() error {
	cmgr.config = ctrl.GetConfigOrDie()

	cmgr.scheme = runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(cmgr.scheme)
	_ = apis.AddToScheme(cmgr.scheme)

	ctrlCache, err := cache.New(cmgr.config, cache.Options{Scheme: cmgr.scheme})
	if err != nil {
		return fmt.Errorf("new ctrl cache: %w", err)
	}
	cmgr.ctrlCache = ctrlCache

	c, err := client.New(cmgr.config, client.Options{Scheme: cmgr.scheme})
	if err != nil {
		return fmt.Errorf("new ctrl client: %w", err)
	}

	cmgr.ctrlClient = c

	cmgr.kubeClient = clientset.NewForConfigOrDie(cmgr.config)

	cmgr.arena, err = arenaclient.NewArenaClient(types.ArenaClientArgs{IsDaemonMode: true, LogLevel: "info"})
	if err != nil {
		return fmt.Errorf("new arena client: %w", err)
	}

	InstallClientManager(cmgr)
	return nil
}

func InitFromManager(mgr ctrl.Manager) {
	cmgr.scheme = mgr.GetScheme()
	cmgr.ctrlCache = mgr.GetCache()
	cmgr.ctrlClient = mgr.GetClient()

	InstallClientManager(cmgr)
}

func Start() {
	cmgr.ctx, cmgr.cancel = context.WithCancel(context.Background())
	go func() {
		cmgr.ctrlCache.Start(cmgr.ctx)
	}()
}

func Shutdown() {
	if cmgr.cancel != nil {
		cmgr.cancel()
	}
}

func (c *clientMgr) IndexField(obj client.Object, field string, extractValue client.IndexerFunc) error {
	return c.ctrlCache.IndexField(context.Background(), obj, field, extractValue)
}

func (c *clientMgr) GetKubeClient() clientset.Interface {
	return c.kubeClient
}

func (c *clientMgr) GetCtrlClient() client.Client {
	return c.ctrlClient
}

func (c *clientMgr) GetCtrlClientWithConfig(kubeConfig []byte) client.Client {
	cfg, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil
	}
	restConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil
	}

	cl, err := client.New(restConfig, client.Options{Scheme: cmgr.scheme})
	if err != nil {
		return nil, fmt.Errorf("new ctrl client with config: %w", err)
	}

	return cl
}

func (c *clientMgr) GetScheme() *runtime.Scheme {
	return c.scheme
}

func (c *clientMgr) GetArenaClient() *arenaclient.ArenaClient {
	return c.arena
}

func (c *clientMgr) GetArenaClientWithConfig(kubeConfigFile string) (*arenaclient.ArenaClient, error) {
	return arenaclient.NewArenaClient(types.ArenaClientArgs{
		Kubeconfig:   kubeConfigFile,
		IsDaemonMode: true,
		LogLevel:     "info",
	})
}
