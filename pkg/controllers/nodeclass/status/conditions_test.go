package nodeclassstatus

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	openshiftkarpenterv1 "github.com/openshift/karpenter-operator/api/karpenter/v1"

	"github.com/awslabs/operatorpkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncConditions(t *testing.T) {
	const generation int64 = 3

	versionResolved := metav1.Condition{
		Type:    openshiftkarpenterv1.ConditionTypeVersionResolved,
		Status:  metav1.ConditionTrue,
		Reason:  openshiftkarpenterv1.ConditionReasonVersionNotSpecified,
		Message: "Using the control plane release image",
	}
	versionNotResolved := metav1.Condition{
		Type:    openshiftkarpenterv1.ConditionTypeVersionResolved,
		Status:  metav1.ConditionFalse,
		Reason:  openshiftkarpenterv1.ConditionReasonResolutionFailed,
		Message: "version 4.30.0 is not available in channel stable-4.21",
	}
	supportedVersionSkew := metav1.Condition{
		Type:    openshiftkarpenterv1.ConditionTypeSupportedVersionSkew,
		Status:  metav1.ConditionTrue,
		Reason:  openshiftkarpenterv1.ConditionReasonAsExpected,
		Message: "Version skew is within the supported policy",
	}
	providerSubnetsReady := status.Condition{
		Type:               "SubnetsReady",
		Status:             metav1.ConditionTrue,
		Reason:             "SubnetsReady",
		ObservedGeneration: 1,
	}
	providerImagesNotReady := status.Condition{
		Type:               "ImagesReady",
		Status:             metav1.ConditionFalse,
		Reason:             "ImagesNotFound",
		Message:            "no images found",
		ObservedGeneration: 1,
	}
	providerReady := status.Condition{
		Type:               openshiftkarpenterv1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             openshiftkarpenterv1.ConditionTypeReady,
		ObservedGeneration: 1,
	}
	providerNotReady := status.Condition{
		Type:               openshiftkarpenterv1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "UnhealthyDependents",
		Message:            "ImagesReady=False",
		ObservedGeneration: 1,
	}

	tests := map[string]struct {
		conditions         []metav1.Condition
		providerConditions []status.Condition
		want               []metav1.Condition
	}{
		"When the version is resolved, it should copy the provider NodeClass conditions including Ready": {
			conditions:         []metav1.Condition{versionResolved, supportedVersionSkew},
			providerConditions: []status.Condition{providerSubnetsReady, providerReady},
			want: []metav1.Condition{
				versionResolved,
				supportedVersionSkew,
				metav1.Condition(providerSubnetsReady),
				metav1.Condition(providerReady),
			},
		},
		"When the provider NodeClass is not ready, it should copy its Ready condition": {
			conditions:         []metav1.Condition{versionResolved},
			providerConditions: []status.Condition{providerImagesNotReady, providerNotReady},
			want: []metav1.Condition{
				versionResolved,
				metav1.Condition(providerImagesNotReady),
				metav1.Condition(providerNotReady),
			},
		},
		"When the version is not resolved, it should set Ready to False with the resolution failure": {
			conditions:         []metav1.Condition{versionNotResolved},
			providerConditions: []status.Condition{providerReady},
			want: []metav1.Condition{
				versionNotResolved,
				{
					Type:               openshiftkarpenterv1.ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             openshiftkarpenterv1.ConditionReasonResolutionFailed,
					Message:            "Version resolution failed: version 4.30.0 is not available in channel stable-4.21",
					ObservedGeneration: generation,
				},
			},
		},
		"When neither the version nor the provider NodeClass are ready, it should prioritize version resolution": {
			conditions:         []metav1.Condition{versionNotResolved},
			providerConditions: []status.Condition{providerImagesNotReady, providerNotReady},
			want: []metav1.Condition{
				versionNotResolved,
				metav1.Condition(providerImagesNotReady),
				{
					Type:               openshiftkarpenterv1.ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             openshiftkarpenterv1.ConditionReasonResolutionFailed,
					Message:            "Version resolution failed: version 4.30.0 is not available in channel stable-4.21",
					ObservedGeneration: generation,
				},
			},
		},
		"When the version resolution is unknown, it should set Ready to False": {
			providerConditions: []status.Condition{providerReady},
			want: []metav1.Condition{
				{
					Type:               openshiftkarpenterv1.ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             openshiftkarpenterv1.ConditionReasonResolutionFailed,
					Message:            "Version resolution status is unknown",
					ObservedGeneration: generation,
				},
			},
		},
		"When the provider NodeClass has no conditions yet, it should keep the existing conditions": {
			conditions: []metav1.Condition{versionResolved, supportedVersionSkew},
			want:       []metav1.Condition{versionResolved, supportedVersionSkew},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			got := SyncConditions(tc.conditions, tc.providerConditions, generation)
			g.Expect(withoutTransitionTimes(got)).To(Equal(tc.want))
		})
	}
}

func TestSyncConditionsDoesNotModifyItsInput(t *testing.T) {
	g := NewWithT(t)
	conditions := []metav1.Condition{{
		Type:   openshiftkarpenterv1.ConditionTypeVersionResolved,
		Status: metav1.ConditionFalse,
		Reason: openshiftkarpenterv1.ConditionReasonResolutionFailed,
	}}
	original := append([]metav1.Condition(nil), conditions...)

	SyncConditions(conditions, []status.Condition{{Type: "SubnetsReady", Status: metav1.ConditionTrue, Reason: "SubnetsReady"}}, 1)

	g.Expect(conditions).To(Equal(original))
}

func TestSyncConditionsKeepsTransitionTimes(t *testing.T) {
	unchanged := metav1.NewTime(time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC))
	providerTransition := metav1.NewTime(time.Date(2026, 9, 30, 11, 0, 0, 0, time.UTC))

	tests := map[string]struct {
		conditions         []metav1.Condition
		providerConditions []status.Condition
	}{
		"When the version stays unresolved while the provider NodeClass is ready, it should not change the Ready condition": {
			conditions: []metav1.Condition{{
				Type:               openshiftkarpenterv1.ConditionTypeReady,
				Status:             metav1.ConditionFalse,
				Reason:             openshiftkarpenterv1.ConditionReasonResolutionFailed,
				Message:            "Version resolution status is unknown",
				ObservedGeneration: 1,
				LastTransitionTime: unchanged,
			}},
			providerConditions: []status.Condition{{
				Type:               openshiftkarpenterv1.ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				Reason:             openshiftkarpenterv1.ConditionTypeReady,
				LastTransitionTime: providerTransition,
			}},
		},
		"When the version is resolved and the provider NodeClass stays ready, it should not change the Ready condition": {
			conditions: []metav1.Condition{
				{
					Type:               openshiftkarpenterv1.ConditionTypeVersionResolved,
					Status:             metav1.ConditionTrue,
					Reason:             openshiftkarpenterv1.ConditionReasonVersionNotSpecified,
					LastTransitionTime: unchanged,
				},
				{
					Type:               openshiftkarpenterv1.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             openshiftkarpenterv1.ConditionTypeReady,
					LastTransitionTime: unchanged,
				},
			},
			providerConditions: []status.Condition{{
				Type:               openshiftkarpenterv1.ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				Reason:             openshiftkarpenterv1.ConditionTypeReady,
				LastTransitionTime: providerTransition,
			}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(SyncConditions(tc.conditions, tc.providerConditions, 1)).To(Equal(tc.conditions))
		})
	}
}

// withoutTransitionTimes clears the transition times, which meta.SetStatusCondition sets to the current time.
func withoutTransitionTimes(conditions []metav1.Condition) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(conditions))
	for _, condition := range conditions {
		condition.LastTransitionTime = metav1.Time{}
		result = append(result, condition)
	}
	return result
}
