package azure

import (
	openshiftkarpenterv1alpha1 "github.com/openshift/karpenter-operator/api/karpenter/v1alpha1"
	"github.com/openshift/karpenter-operator/pkg/assets"
	"github.com/openshift/karpenter-operator/pkg/cloudprovider/common"
	azurenodeclass "github.com/openshift/karpenter-operator/pkg/controllers/nodeclass/azure"
	azurevap "github.com/openshift/karpenter-operator/pkg/controllers/nodeclass/azure/vap"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ common.HCPNodeClassProvider = hcpAzureNodeClassProvider{}

type hcpAzureNodeClassProvider struct {
	vnetSubnetID string
}

// DefaultNodeClass returns the default Azure NodeClass and mutation function for its complete desired state.
func (p hcpAzureNodeClassProvider) DefaultNodeClass(_ string) (client.Object, controllerutil.MutateFn, error) {
	nodeClass := &openshiftkarpenterv1alpha1.OpenShiftAzureNodeClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openshiftkarpenterv1alpha1.SchemeGroupVersion.String(),
			Kind:       "OpenShiftAzureNodeClass",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
	}

	mutate := func() error {
		nodeClass.Labels = map[string]string{
			"app.kubernetes.io/managed-by": "karpenter-operator",
		}
		nodeClass.Spec = openshiftkarpenterv1alpha1.OpenShiftAzureNodeClassSpec{
			VNETSubnetID: p.vnetSubnetID,
		}

		return nil
	}
	return nodeClass, mutate, nil
}

// WatchObject returns the Azure NodeClass type watched for default NodeClass changes.
func (hcpAzureNodeClassProvider) WatchObject() client.Object {
	return &openshiftkarpenterv1alpha1.OpenShiftAzureNodeClass{}
}

// CRDs returns the OpenShiftAzureNodeClass CRD.
func (hcpAzureNodeClassProvider) CRDs() []*apiextensionsv1.CustomResourceDefinition {
	return assets.AzureHCPCRDs
}

// NewControllers returns the controllers that reconcile OpenShiftAzureNodeClass and AKSNodeClass resources.
func (hcpAzureNodeClassProvider) NewControllers(hostedCluster cluster.Cluster, namespace string) []common.NodeClassController {
	return []common.NodeClassController{
		azurenodeclass.NewNodeClassController(hostedCluster, namespace),
		azurevap.NewController(hostedCluster),
	}
}
