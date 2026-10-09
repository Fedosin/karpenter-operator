package ec2nodeclass

import (
	"testing"

	. "github.com/onsi/gomega"

	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	awskarpenterv1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAMISelectorTermsFromUserDataSecret(t *testing.T) {
	tests := map[string]struct {
		userDataSecret *corev1.Secret
		expectedError  string
		expectedAMIs   []awskarpenterv1.AMISelectorTerm
	}{
		"When AMI labels exist, it should return the expected AMIs": {
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-data-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						archToAMILabelKey(hyperv1beta1.ArchitectureAMD64): "ami-123",
						archToAMILabelKey(hyperv1beta1.ArchitectureARM64): "ami-456",
					},
				},
			},
			expectedAMIs: []awskarpenterv1.AMISelectorTerm{
				{
					ID: "ami-123",
				},
				{
					ID: "ami-456",
				},
			},
		},
		"When no AMI labels exist, it should return an error": {
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-data-secret",
					Namespace: "test-namespace",
					Labels:    map[string]string{},
				},
			},
			expectedError: "no AMIs found for supported architectures: [amd64 arm64]",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			amis, err := amiSelectorTermsFromUserDataSecret(tc.userDataSecret)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tc.expectedError))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(amis).To(Equal(tc.expectedAMIs))
		})
	}
}
