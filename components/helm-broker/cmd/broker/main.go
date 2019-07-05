package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	scCs "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/kyma-project/kyma/components/cms-controller-manager/pkg/apis/cms/v1alpha1"
	"github.com/kyma-project/kyma/components/helm-broker/internal/bind"
	"github.com/kyma-project/kyma/components/helm-broker/internal/broker"
	"github.com/kyma-project/kyma/components/helm-broker/internal/bundle"
	"github.com/kyma-project/kyma/components/helm-broker/internal/config"
	"github.com/kyma-project/kyma/components/helm-broker/internal/helm"
	"github.com/kyma-project/kyma/components/helm-broker/internal/storage"
	"github.com/kyma-project/kyma/components/helm-broker/platform/logger"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
)

const (
	mapLabelKey   = "helm-broker-repo"
	mapLabelValue = "true"
)

func main() {
	verbose := flag.Bool("verbose", false, "specify if log verbosely loading configuration")
	flag.Parse()
	cfg, err := config.Load(*verbose)
	fatalOnError(err)

	// creates the in-cluster k8sConfig
	k8sConfig, err := newRestClientConfig(cfg.KubeconfigPath)
	fatalOnError(err)

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	fatalOnError(err)

	log := logger.New(&cfg.Logger)



	// ServiceCatalog
	scClientSet, err := scCs.NewForConfig(k8sConfig)
	fatalOnError(err)
	csbInterface := scClientSet.ServicecatalogV1beta1().ClusterServiceBrokers()



	sch, err := v1alpha1.SchemeBuilder.Build()
	fatalOnError(err)

	dynamicClient, err := client.New(k8sConfig, client.Options{Scheme: sch})
	fatalOnError(err)
	helmClient := helm.NewClient(cfg.Helm, log)

	stopCh := make(chan struct{})
	srv, brokerSyncer := SetupSeverAndRunControllers(cfg, dynamicClient, clientset, csbInterface, helmClient, stopCh, log)


	startedCh := make(chan struct{})
	go func() {
		// wait for server HTTP to be ready
		<-startedCh
		log.Infof("Waiting for service %s to be ready", cfg.HelmBrokerURL)

		// Running Helm Broker does not mean it is visible to the service catalog
		// This is the reason of the check cfg.HelmBrokerURL
		waitForHelmBrokerIsReady(cfg.HelmBrokerURL, 15*time.Second, log)
		log.Infof("%s service ready", cfg.HelmBrokerURL)

		err := brokerSyncer.Sync(cfg.ClusterServiceBrokerName, 5)
		if err != nil {
			log.Errorf("Could not synchronize Service Catalog with the broker: %s", err)
		}
	}()

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	cancelOnChanInterrupt(ctx, stopCh, cancelFunc)
	cancelOnInterrupt(ctx, cancelFunc)

	err = srv.Run(ctx, fmt.Sprintf(":%d", cfg.Port), startedCh)
	fatalOnError(err)
}

func SetupSeverAndRunControllers(cfg *config.Config, dynamicClient client.Client, clientset kubernetes.Interface, csbInterface v1beta1.ClusterServiceBrokerInterface, helmClient *helm.Client, stopCh chan struct{}, log *logrus.Entry) (*broker.Server, *broker.ClusterServiceBrokerSync) {
	storageConfig := storage.ConfigList(cfg.Storage)
	sFact, err := storage.NewFactory(&storageConfig)
	fatalOnError(err)

	bLoader := bundle.NewLoader(cfg.TmpDir, log)

	docsProvider := bundle.NewDocsProvider(dynamicClient)
	bundleSyncer := bundle.NewSyncer(sFact.Bundle(), sFact.Chart(), docsProvider, log)

	brokerSyncer := broker.NewClusterServiceBrokerSyncer(csbInterface, log)

	cfgMapInformer := v1.NewFilteredConfigMapInformer(clientset, cfg.Namespace, 10*time.Minute, cache.Indexers{}, func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("%s=%s", mapLabelKey, mapLabelValue)
	})

	repositoryWatcher := bundle.NewRepositoryController(bundleSyncer, bLoader, brokerSyncer, cfg.ClusterServiceBrokerName, cfgMapInformer, log, cfg.DevelopMode)
	go repositoryWatcher.Run(stopCh)
	go cfgMapInformer.Run(stopCh)


	srv := broker.New(sFact.Bundle(), sFact.Chart(), sFact.InstanceOperation(), sFact.Instance(), sFact.InstanceBindData(),
		bind.NewRenderer(), bind.NewResolver(clientset.CoreV1()), helmClient, bundleSyncer, log)

	return srv, brokerSyncer
}

func waitForHelmBrokerIsReady(url string, timeout time.Duration, log logrus.FieldLogger) {
	timeoutCh := time.After(timeout)
	for {
		r, err := http.Get(fmt.Sprintf("%s/statusz", url))
		if err == nil {
			// no need to read the response
			ioutil.ReadAll(r.Body)
			r.Body.Close()
		}
		if err == nil && r.StatusCode == http.StatusOK {
			break
		}

		select {
		case <-timeoutCh:
			log.Errorf("Waiting for service %s to be ready timeout %s exceeded.", url, timeout.String())
			if err != nil {
				log.Errorf("Last call error: %s", err.Error())
			} else {
				log.Errorf("Last call response status: %s", r.StatusCode)
			}
			return
		default:
			time.Sleep(time.Second)
		}
	}
}

func fatalOnError(err error) {
	if err != nil {
		logrus.Fatal(err.Error())
	}
}

// cancelOnInterrupt calls cancel func when os.Interrupt or SIGTERM is received
func cancelOnInterrupt(ctx context.Context, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case <-c:
			cancel()
		}
	}()
}

// cancelOnInterrupt closes given channel and also calls cancel func when os.Interrupt or SIGTERM is received
func cancelOnChanInterrupt(ctx context.Context, ch chan<- struct{}, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
			close(ch)
		case <-c:
			close(ch)
			cancel()
		}
	}()
}

func newRestClientConfig(kubeConfigPath string) (*rest.Config, error) {
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}

	return rest.InClusterConfig()
}
