package controller

import (
	"os"

	scCs "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"github.com/kyma-project/kyma/components/helm-broker/internal/controller/broker"
	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/kyma-project/kyma/components/helm-broker/internal/bundle"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kyma-project/kyma/components/cms-controller-manager/pkg/apis/cms/v1alpha1"
	"github.com/kyma-project/kyma/components/helm-broker/internal/config"
	"github.com/kyma-project/kyma/components/helm-broker/internal/storage"
	"github.com/kyma-project/kyma/components/helm-broker/pkg/apis"
)

func SetupAndStartController(cfg *rest.Config, ctrCfg *config.ControllerConfig, metricsAddr string, sFact storage.Factory, log *logrus.Entry) manager.Manager {
	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: metricsAddr})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up schemes")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add Addons APIs to scheme")
		os.Exit(1)
	}
	if err := v1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add ServiceCatalog APIs to scheme")
		os.Exit(1)
	}
	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add CMS APIs to scheme")
		os.Exit(1)
	}

	// TODO: use generic client
	scClientSet, err := scCs.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create Service Catalog client")
		os.Exit(1)
	}

	dynamicClient, err := client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		log.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}

	docsProvider := NewDocsProvider(dynamicClient)
	brokerSyncer := broker.NewServiceBrokerSyncer(scClientSet.ServicecatalogV1beta1(), scClientSet.ServicecatalogV1beta1(), ctrCfg.ClusterServiceBrokerName, log)
	sbFacade := broker.NewBrokersFacade(scClientSet.ServicecatalogV1beta1(), brokerSyncer, ctrCfg.Namespace, ctrCfg.ServiceName)
	csbFacade := broker.NewClusterBrokersFacade(scClientSet.ServicecatalogV1beta1(), brokerSyncer, ctrCfg.Namespace, ctrCfg.ServiceName, ctrCfg.ClusterServiceBrokerName)

	bundleProvider := bundle.NewProvider(bundle.NewHTTPRepository(), bundle.NewLoader(ctrCfg.TmpDir, log), log)

	log.Info("Setting up controller")
	acReconcile := NewReconcileAddonsConfiguration(mgr, bundleProvider, sbFacade, sFact.Chart(), sFact.Bundle(), ctrCfg.DevelopMode, docsProvider, brokerSyncer)
	acController := NewAddonsConfigurationController(acReconcile)
	err = acController.Start(mgr)
	if err != nil {
		log.Error(err, "unable to start AddonsConfigurationController")
	}
	cacReconcile := NewReconcileClusterAddonsConfiguration(mgr, bundleProvider, sFact.Chart(), sFact.Bundle(), csbFacade, docsProvider, brokerSyncer, ctrCfg.DevelopMode)
	cacController := NewClusterAddonsConfigurationController(cacReconcile)
	err = cacController.Start(mgr)

	if err != nil {
		log.Error(err, "unable to start ClusterAddonsConfigurationController")
	}
	return mgr
}
