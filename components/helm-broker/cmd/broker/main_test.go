package main

import (
	"testing"
	"net/http/httptest"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"time"
	"github.com/stretchr/testify/assert"
	"fmt"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/kyma-project/kyma/components/helm-broker/pkg/apis/addons/v1alpha1"
	"net/http"
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/kyma-project/kyma/components/helm-broker/internal/config"
	"os"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	k8s "k8s.io/client-go/kubernetes"
	"github.com/kyma-project/kyma/components/helm-broker/internal/storage/testdata"
	"github.com/kyma-project/kyma/components/helm-broker/platform/logger/spy"
	v12 "k8s.io/api/core/v1"

	scFake "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/sirupsen/logrus"
)

const (
	bundleID = "id-09834-abcd-234"
)

func TestGetClusterCatalogHappyPath(t *testing.T) {
	// given
	suite := newTestSuite(t)
	defer suite.tearDown()
	suite.AssertNoServicesInCatalogEndpoint("cluster")

	// when
	suite.createClusterAddonsConfiguration()

	// then
	//suite.WaitForClusterAddonsConfigurationStatusReady()
	suite.AssertServicesInCatalogEndpoint("cluster")

}

func TestGetNamespacedCatalogHappyPath(t *testing.T) {
	// given
	suite := newTestSuite(t)
	defer suite.tearDown()
	suite.AssertNoServicesInCatalogEndpoint("ns/stage")

	// when
	suite.createAddonsConfiguration("stage")

	// then
	suite.WaitForAddonsConfigurationStatusReady("stage")
	suite.AssertServicesInCatalogEndpoint("ns/stage")
}

func newTestSuite(t *testing.T) *testSuite {
	_ = spy.NewLogSink()
	stopCh := make(chan struct{})

	sch, err := v1alpha1.SchemeBuilder.Build()
	require.NoError(t, err)
	cli := fake.NewFakeClientWithScheme(sch)
	k8sClientset := kubernetes.NewSimpleClientset()
	scClient := scFake.NewSimpleClientset()
	csbClient := scClient.ServicecatalogV1beta1().ClusterServiceBrokers()

	brokerServer, _ := SetupSeverAndRunControllers(&config.Config{
		TmpDir:      os.TempDir(),
		Namespace:   "kyma-system",
		Storage:     testdata.GoldenConfigMemorySingleAll(),
		DevelopMode: true,
		ClusterServiceBrokerName: "helm-broker", //todo: remove
	}, cli, k8sClientset, csbClient, nil, stopCh, logrus.New().WithField("", ""))

	server := httptest.NewServer(brokerServer.CreateHandler())
	repoServer := httptest.NewServer(http.FileServer(http.Dir("testdata")))

	// todo: remove
	csbClient.Create(&v1beta1.ClusterServiceBroker{
		ObjectMeta: v1.ObjectMeta{
			Name: "helm-broker",
		},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			CommonServiceBrokerSpec: v1beta1.CommonServiceBrokerSpec{
				URL: server.URL,
			},
		},
	})

	return &testSuite{
		t: t,

		dynamicClient: cli,
		repoServer:    repoServer,
		server:        server,
		k8sClient:     k8sClientset,
	}
}

func newOSBClient(url string) (osb.Client, error) {
	config := osb.DefaultClientConfiguration()
	config.URL = url
	config.APIVersion = osb.Version2_13()

	osbClient, err := osb.NewClient(config)
	if err != nil {
		return nil, err
	}

	return osbClient, nil
}

type testSuite struct {
	t          *testing.T
	server     *httptest.Server
	repoServer *httptest.Server

	osbClient     osb.Client
	dynamicClient client.Client
	k8sClient     k8s.Interface
}

func (ts *testSuite) tearDown() {
	ts.server.Close()
	ts.repoServer.Close()
}

func (ts *testSuite) AssertNoServicesInCatalogEndpoint(prefix string) {
	osbClient, err := newOSBClient(fmt.Sprintf("%s/%s", ts.server.URL, prefix))
	require.NoError(ts.t, err)
	resp, err := osbClient.GetCatalog()
	require.NoError(ts.t, err)

	assert.Empty(ts.t, resp.Services)
}

func (ts *testSuite) AssertServicesInCatalogEndpoint(prefix string) {
	osbClient, err := newOSBClient(fmt.Sprintf("%s/%s", ts.server.URL, prefix))
	require.NoError(ts.t, err)

	timeoutCh := time.After(3 * time.Second)
	for {
		err := ts.checkServiceID(osbClient)
		if err == nil {
			return
		}
		select {
		case <-timeoutCh:
			assert.Failf(ts.t, "The timeout exceeded while waiting for the OSB catalog response, last error: %s", err.Error())
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (ts *testSuite) checkServiceID(osbClient osb.Client) error {
	osbResponse, err := osbClient.GetCatalog()
	if err != nil {
		return err
	}

	if len(osbResponse.Services) == 1 && osbResponse.Services[0].ID == bundleID {
		return nil
	}

	return fmt.Errorf("unexpected GetCatalogResponse %v", osbResponse)
}

func (ts *testSuite) createClusterAddonsConfiguration() {
	// todo: remove it
	ts.k8sClient.CoreV1().ConfigMaps("kyma-system").Create(&v12.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string {
				"helm-broker-repo": "true",
			},
		},
		Data: map[string]string {
			"URLs": ts.repoServer.URL + "/index.yaml",
		},
	})

	ts.dynamicClient.Create(context.TODO(), &v1alpha1.ClusterAddonsConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: "addons",
		},
		Spec: v1alpha1.ClusterAddonsConfigurationSpec{
			CommonAddonsConfigurationSpec: v1alpha1.CommonAddonsConfigurationSpec{
				Repositories: []v1alpha1.SpecRepository{
					{URL: ts.repoServer.URL + "/index.yaml"},
				},
			},
		},
	})
}

func (ts *testSuite) WaitForClusterAddonsConfigurationStatusReady() {
	var cac v1alpha1.ClusterAddonsConfiguration
	ts.waitForReady(&cac, &(cac.Status.CommonAddonsConfigurationStatus), types.NamespacedName{Name: "addons"})
}

func (ts *testSuite) WaitForAddonsConfigurationStatusReady(namespace string) {
	var cac v1alpha1.ClusterAddonsConfiguration
	ts.waitForReady(&cac, &(cac.Status.CommonAddonsConfigurationStatus), types.NamespacedName{Name: "addons", Namespace: namespace})
}

func (ts *testSuite) waitForReady(obj runtime.Object, status *v1alpha1.CommonAddonsConfigurationStatus, nn types.NamespacedName) {
	timeoutCh := time.After(3 * time.Second)
	for {
		err := ts.dynamicClient.Get(context.TODO(), nn, obj)
		require.NoError(ts.t, err)

		if status.Phase == v1alpha1.AddonsConfigurationReady {
			return
		}

		select {
		case <-timeoutCh:
			assert.Fail(ts.t, "The timeout exceeded while waiting for the Phase Ready, current phase: ", string(status.Phase))
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (ts *testSuite) createAddonsConfiguration(namespace string) {
	ts.dynamicClient.Create(context.TODO(), &v1alpha1.ClusterAddonsConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:      "addons",
			Namespace: namespace,
		},
		Spec: v1alpha1.ClusterAddonsConfigurationSpec{
			CommonAddonsConfigurationSpec: v1alpha1.CommonAddonsConfigurationSpec{
				Repositories: []v1alpha1.SpecRepository{
					{URL: ts.repoServer.URL + "/index.yaml"},
				},
			},
		},
	})
}

// todo: remove it
type dummyBroker struct {
}

func (*dummyBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"services":[]}`))
}
