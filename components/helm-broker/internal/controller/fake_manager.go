package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	runtimeTypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type fakeManager struct {
	t      *testing.T
	client client.Client
	sch    *runtime.Scheme
}

func (fakeManager) Add(manager.Runnable) error {
	return nil
}

func (fm fakeManager) SetFields(i interface{}) error {
	//if _, err := inject.ConfigInto(cm.config, i); err != nil {
	//return err
	//}
	if _, err := inject.ClientInto(fm.client, i); err != nil {
		return err
	}
	if _, err := inject.SchemeInto(fm.sch, i); err != nil {
		return err
	}
	//if _, err := inject.CacheInto(cm.cache, i); err != nil {
	//return err
	//}
	if _, err := inject.InjectorInto(fm.SetFields, i); err != nil {
		return err
	}
	//if _, err := inject.StopChannelInto(cm.internalStop, i); err != nil {
	//return err
	//}
	//if _, err := inject.DecoderInto(cm.admissionDecoder, i); err != nil {
	//return err
	//}
	return nil
}

func (fakeManager) Start(<-chan struct{}) error {
	return nil
}

func (fakeManager) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (f *fakeManager) GetScheme() *runtime.Scheme {
	// Setup schemes for all resources
	return f.sch
}

func (fakeManager) GetAdmissionDecoder() runtimeTypes.Decoder {
	return nil
}

func (f *fakeManager) GetClient() client.Client {
	return f.client
}

func (fakeManager) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func (fakeManager) GetCache() cache.Cache {
	return nil
}

func (fakeManager) GetRecorder(name string) record.EventRecorder {
	return nil
}

func (fakeManager) GetRESTMapper() meta.RESTMapper {
	return nil
}

func NewFakeManager(t *testing.T, cli client.Client, sch *runtime.Scheme) manager.Manager {
	return &fakeManager{
		t:      t,
		client: cli,
		sch:    sch,
	}
}
