package hypershift

import (
	configv1 "github.com/openshift/api/config/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	"k8s.io/apimachinery/pkg/api/equality"
)

// WaitsForControlPlaneUpgrade reports whether a NodeClass with the given spec.version waits for the control plane
// upgrade to complete before its nodes move to a new release. A NodeClass without a version follows the last
// completed control plane release, so it waits while the control plane upgrades. A NodeClass with a version never
// follows the control plane.
func WaitsForControlPlaneUpgrade(hcp *hyperv1beta1.HostedControlPlane, version string) bool {
	return version == "" && isControlPlaneUpgrading(hcp)
}

// isControlPlaneUpgrading returns true when the desired release image differs
// from the most recent Completed version in history.
// Returns false during initial install (no Completed entry or desired not yet populated).
// The loop returns on the first CompletedUpdate entry found. This is safe because the
// ControlPlaneVersion.History list is ordered newest-first per the API contract.
func isControlPlaneUpgrading(hcp *hyperv1beta1.HostedControlPlane) bool {
	desiredImage := hcp.Status.ControlPlaneVersion.Desired.Image
	if desiredImage == "" {
		return false
	}

	for _, entry := range hcp.Status.ControlPlaneVersion.History {
		if entry.State == configv1.CompletedUpdate {
			return entry.Image != desiredImage
		}
	}

	return false
}

// ControlPlaneVersionChanged reports whether the control plane version status differs between two HostedControlPlanes,
// e.g. because an upgrade started or completed.
func ControlPlaneVersionChanged(oldHCP, newHCP *hyperv1beta1.HostedControlPlane) bool {
	return !equality.Semantic.DeepEqual(oldHCP.Status.ControlPlaneVersion, newHCP.Status.ControlPlaneVersion)
}
