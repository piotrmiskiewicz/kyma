package populator

import (
	"github.com/kyma-project/kyma/components/remote-environment-broker/internal"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
)

func mapServiceInstance(in *v1beta1.ServiceInstance) *internal.Instance {
	var state internal.InstanceState

	if isServiceInstanceReady(in) {
		state = internal.InstanceStateSucceeded
	} else {
		state = internal.InstanceStateFailed
	}

	return &internal.Instance{
		ID:            internal.InstanceID(in.Spec.ExternalID),
		Namespace:     internal.Namespace(in.Namespace),
		ParamsHash:    "TODO",
		ServicePlanID: internal.ServicePlanID(in.Spec.ClusterServicePlanRef.Name),
		ServiceID:     internal.ServiceID(in.Spec.ClusterServiceClassRef.Name),
		State:         state,
	}
}


func isServiceInstanceReady(instance *v1beta1.ServiceInstance) bool {
	for _, cond := range instance.Status.Conditions {
		if cond.Type == v1beta1.ServiceInstanceConditionReady {
			return cond.Status == v1beta1.ConditionTrue
		}
	}
	return false
}