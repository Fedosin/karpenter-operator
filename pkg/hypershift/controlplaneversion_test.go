package hypershift

import (
	"testing"

	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

func TestWaitsForControlPlaneUpgrade(t *testing.T) {
	const (
		completedImage = "quay.io/openshift-release-dev/ocp-release:4.21.10-x86_64"
		desiredImage   = "quay.io/openshift-release-dev/ocp-release:4.22.1-x86_64"
	)

	upgradingVersion := hyperv1beta1.ControlPlaneVersionStatus{
		Desired: configv1.Release{Image: desiredImage},
		History: []hyperv1beta1.ControlPlaneUpdateHistory{
			{State: configv1.PartialUpdate, Image: desiredImage},
			{State: configv1.CompletedUpdate, Image: completedImage},
		},
	}

	tests := map[string]struct {
		controlPlaneVersion hyperv1beta1.ControlPlaneVersionStatus
		version             string
		expected            bool
	}{
		"When the desired image is empty, it should return false": {
			expected: false,
		},
		"When no history entries exist (initial install), it should return false": {
			controlPlaneVersion: hyperv1beta1.ControlPlaneVersionStatus{
				Desired: configv1.Release{Image: completedImage},
			},
			expected: false,
		},
		"When only partial entries exist (initial install still running), it should return false": {
			controlPlaneVersion: hyperv1beta1.ControlPlaneVersionStatus{
				Desired: configv1.Release{Image: completedImage},
				History: []hyperv1beta1.ControlPlaneUpdateHistory{
					{State: configv1.PartialUpdate, Image: completedImage},
				},
			},
			expected: false,
		},
		"When the desired image matches the last completed image, it should return false": {
			controlPlaneVersion: hyperv1beta1.ControlPlaneVersionStatus{
				Desired: configv1.Release{Image: completedImage},
				History: []hyperv1beta1.ControlPlaneUpdateHistory{
					{State: configv1.CompletedUpdate, Image: completedImage},
				},
			},
			expected: false,
		},
		"When the control plane is upgrading and the NodeClass has no version, it should return true": {
			controlPlaneVersion: upgradingVersion,
			expected:            true,
		},
		"When the control plane is upgrading and the NodeClass has a version, it should return false": {
			controlPlaneVersion: upgradingVersion,
			version:             "4.21.10",
			expected:            false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			hcp := &hyperv1beta1.HostedControlPlane{
				Status: hyperv1beta1.HostedControlPlaneStatus{ControlPlaneVersion: tc.controlPlaneVersion},
			}
			g.Expect(WaitsForControlPlaneUpgrade(hcp, tc.version)).To(Equal(tc.expected))
		})
	}
}

func TestControlPlaneVersionChanged(t *testing.T) {
	const (
		completedImage = "quay.io/openshift-release-dev/ocp-release:4.21.10-x86_64"
		desiredImage   = "quay.io/openshift-release-dev/ocp-release:4.22.1-x86_64"
	)

	upgrading := hyperv1beta1.ControlPlaneVersionStatus{
		Desired: configv1.Release{Image: desiredImage},
		History: []hyperv1beta1.ControlPlaneUpdateHistory{
			{State: configv1.PartialUpdate, Image: desiredImage},
			{State: configv1.CompletedUpdate, Image: completedImage},
		},
	}
	upgraded := hyperv1beta1.ControlPlaneVersionStatus{
		Desired: configv1.Release{Image: desiredImage},
		History: []hyperv1beta1.ControlPlaneUpdateHistory{
			{State: configv1.CompletedUpdate, Image: desiredImage},
			{State: configv1.CompletedUpdate, Image: completedImage},
		},
	}

	tests := map[string]struct {
		oldVersion hyperv1beta1.ControlPlaneVersionStatus
		newVersion hyperv1beta1.ControlPlaneVersionStatus
		expected   bool
	}{
		"When a control plane upgrade completes, it should return true": {
			oldVersion: upgrading,
			newVersion: upgraded,
			expected:   true,
		},
		"When the control plane version does not change, it should return false": {
			oldVersion: upgraded,
			newVersion: upgraded,
			expected:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			oldHCP := &hyperv1beta1.HostedControlPlane{Status: hyperv1beta1.HostedControlPlaneStatus{ControlPlaneVersion: tc.oldVersion}}
			newHCP := &hyperv1beta1.HostedControlPlane{Status: hyperv1beta1.HostedControlPlaneStatus{ControlPlaneVersion: tc.newVersion}}
			g.Expect(ControlPlaneVersionChanged(oldHCP, newHCP)).To(Equal(tc.expected))
		})
	}
}
