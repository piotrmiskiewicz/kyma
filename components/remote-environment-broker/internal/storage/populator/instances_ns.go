package populator

import (
	"context"
	"time"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	scv1beta "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	listersv1beta "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/v1beta1"
	"github.com/kyma-project/kyma/components/remote-environment-broker/internal"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"github.com/kyma-project/kyma/components/remote-environment-broker/internal/nsbroker"
)

// NamespacedInstances provide method for populating Instance storage created from namespaced classes
type NamespacedInstances struct {
	inserter          instanceInserter
	scClientSet       clientset.Interface
	namespace         string
}

// NewNamespacedInstances is a constructor of Instances populator
func NewNamespacedInstances(scClientSet clientset.Interface, inserter instanceInserter, namespace string, clusterScoped bool) *NamespacedInstances {
	return &NamespacedInstances{
		scClientSet: scClientSet,
		inserter:    inserter,
		namespace:   namespace,
	}
}

// Do perform instances population
func (p *NamespacedInstances) Do(ctx context.Context) error {
	siInformer := scv1beta.NewServiceInstanceInformer(p.scClientSet, v1.NamespaceAll, informerResyncPeriod, nil)
	scInformer := scv1beta.NewServiceClassInformer(p.scClientSet, p.namespace, informerResyncPeriod, nil)

	go siInformer.Run(ctx.Done())
	go scInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), siInformer.HasSynced) {
		return errors.New("cannot synchronize service instance cache")
	}

	if !cache.WaitForCacheSync(ctx.Done(), scInformer.HasSynced) {
		return errors.New("cannot synchronize service class cache")
	}

	scLister := listersv1beta.NewServiceClassLister(scInformer.GetIndexer())
	serviceClasses, err := scLister.ServiceClasses(p.namespace).List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "while listing service classes")
	}

	rebClassNames := make(map[string]struct{})
	for _, sc := range serviceClasses {
		if sc.Spec.ServiceBrokerName == nsbroker.BrokerName {
			rebClassNames[sc.Name] = struct{}{}
		}
	}

	siLister := listersv1beta.NewServiceInstanceLister(siInformer.GetIndexer())
	serviceInstances, err := siLister.List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "while listing service instances")
	}

	for _, si := range serviceInstances {
		if _, ex := rebClassNames[si.Spec.ServiceClassRef.Name]; ex {
			if err := p.inserter.Insert(mapServiceInstance(si)); err != nil {
				return errors.Wrap(err, "while inserting service instance")
			}
		}
	}
	return nil
}
