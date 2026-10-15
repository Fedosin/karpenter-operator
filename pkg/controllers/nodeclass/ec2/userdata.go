package ec2nodeclass

import (
	"fmt"

	openshiftkarpenterv1 "github.com/openshift/karpenter-operator/api/karpenter/v1"

	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	awskarpenterv1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	corev1 "k8s.io/api/core/v1"
)

// supportedArchitectures lists the architectures with AMIs for HCP AWS.
var supportedArchitectures = []string{hyperv1beta1.ArchitectureAMD64, hyperv1beta1.ArchitectureARM64}

// amiSelectorTermsFromUserDataSecret returns the AMI selector terms for all supported architectures
// that have an AMI label on the user data Secret.
func amiSelectorTermsFromUserDataSecret(userDataSecret *corev1.Secret) ([]awskarpenterv1.AMISelectorTerm, error) {
	terms := []awskarpenterv1.AMISelectorTerm{}
	for _, arch := range supportedArchitectures {
		labelKey := archToAMILabelKey(arch)
		if ami, ok := userDataSecret.Labels[labelKey]; ok {
			terms = append(terms, awskarpenterv1.AMISelectorTerm{ID: ami})
		}
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("no AMIs found for supported architectures: %v", supportedArchitectures)
	}
	return terms, nil
}

// archToAMILabelKey returns the user data Secret label key that stores the AMI ID for the given architecture.
func archToAMILabelKey(arch string) string {
	if arch == hyperv1beta1.ArchitectureAMD64 {
		return openshiftkarpenterv1.UserDataAMILabel
	}
	return fmt.Sprintf("%s-%s", openshiftkarpenterv1.UserDataAMILabel, arch)
}
