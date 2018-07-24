// Code generated by mockery v1.0.0
package automock

import controller "github.com/kyma-project/kyma/components/binding-usage-controller/internal/controller"
import mock "github.com/stretchr/testify/mock"

// SupervisorRegistry is an autogenerated mock type for the SupervisorRegistry type
type SupervisorRegistry struct {
	mock.Mock
}

// Register provides a mock function with given fields: k, supervisor
func (_m *SupervisorRegistry) Register(k controller.Kind, supervisor controller.KubernetesResourceSupervisor) error {
	ret := _m.Called(k, supervisor)

	var r0 error
	if rf, ok := ret.Get(0).(func(controller.Kind, controller.KubernetesResourceSupervisor) error); ok {
		r0 = rf(k, supervisor)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Unregister provides a mock function with given fields: k
func (_m *SupervisorRegistry) Unregister(k controller.Kind) error {
	ret := _m.Called(k)

	var r0 error
	if rf, ok := ret.Get(0).(func(controller.Kind) error); ok {
		r0 = rf(k)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
