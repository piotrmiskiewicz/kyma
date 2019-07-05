// +build integration

package integration_test

import "testing"

const (
	// bundleID is the ID of the bundle redis in testdata dir
	bundleID = "id-09834-abcd-234"

	addonsConfigName = "addons"
)

func TestGetClusterCatalogHappyPath(t *testing.T) {
	// given
	suite := newTestSuite(t)
	defer suite.tearDown()
	suite.AssertNoServicesInCatalogEndpoint("cluster")

	// when
	suite.createClusterAddonsConfiguration()

	// then
	suite.WaitForClusterAddonsConfigurationStatusReady()
	suite.WaitForServicesInCatalogEndpoint("cluster")

	// when
	suite.removeRepoFromClusterAddonsConfiguration("stage")

	// then
	suite.WaitForEmptyCatalogResponse("cluster")
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
	suite.WaitForServicesInCatalogEndpoint("ns/stage")

	// when
	suite.removeRepoFromAddonsConfiguration("stage")

	// then
	suite.WaitForEmptyCatalogResponse("ns/stage")
}
