package hypershift

import (
	"context"
	"fmt"

	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var capiClusterGroupKind = schema.GroupKind{Group: "cluster.x-k8s.io", Kind: "Cluster"}

// ResolveCAPIClusterGVK returns the CAPI Cluster version preferred by the API server.
// Only CAPI Cluster metadata is read, which is identical across versions, so no specific
// version has to be required.
func ResolveCAPIClusterGVK(mapper meta.RESTMapper) (schema.GroupVersionKind, error) {
	mapping, err := mapper.RESTMapping(capiClusterGroupKind)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("resolving CAPI Cluster version: %w", err)
	}
	return mapping.GroupVersionKind, nil
}

// NewCAPIClusterMetadata returns an empty metadata-only CAPI Cluster for use with Get and watches.
func NewCAPIClusterMetadata(gvk schema.GroupVersionKind) *metav1.PartialObjectMetadata {
	capiCluster := &metav1.PartialObjectMetadata{}
	capiCluster.SetGroupVersionKind(gvk)
	return capiCluster
}

// IsClusterDeleting returns true when the cluster is being torn down.
// It checks both the HCP DeletionTimestamp and the CAPI Cluster DeletionTimestamp.
// The CAPI Cluster is deleted earlier in the HostedCluster deletion sequence than
// the HCP, so checking it allows node cleanup to begin sooner — in parallel with
// regular CAPI node teardown instead of after it completes.
func IsClusterDeleting(ctx context.Context, c client.Reader, hcp *hyperv1beta1.HostedControlPlane,
	capiClusterGVK schema.GroupVersionKind) (bool, error) {
	if hcp.DeletionTimestamp != nil {
		return true, nil
	}

	if hcp.Spec.InfraID == "" {
		return false, nil
	}

	capiCluster := NewCAPIClusterMetadata(capiClusterGVK)
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: hcp.Namespace,
		Name:      hcp.Spec.InfraID,
	}, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting CAPI Cluster: %w", err)
	}

	return !capiCluster.DeletionTimestamp.IsZero(), nil
}
