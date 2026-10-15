package hypershift

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	openshiftkarpenterv1 "github.com/openshift/karpenter-operator/api/karpenter/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/onsi/gomega/types"
)

const hcpNamespace = "clusters-example"

func TestGetUserDataSecret(t *testing.T) {
	tests := map[string]struct {
		objects        []client.Object
		expectedError  types.GomegaMatcher
		expectedSecret string
	}{
		"When the user data Secret exists, it should return it": {
			objects:        []client.Object{newUserDataSecret("default", "3f1c9a2b")},
			expectedSecret: "user-data-default-karpenter-3f1c9a2b",
		},
		"When other Karpenter Secrets exist, it should return the user data Secret of the NodeClass": {
			objects: []client.Object{
				newUserDataSecret("gpu", "7d0e4c6f"),
				newTokenSecret("default", "3f1c9a2b"),
				newUserDataSecret("default", "3f1c9a2b"),
			},
			expectedSecret: "user-data-default-karpenter-3f1c9a2b",
		},
		"When several user data Secrets exist, it should return an error": {
			objects: []client.Object{
				newUserDataSecret("default", "3f1c9a2b"),
				newUserDataSecret("default", "7d0e4c6f"),
			},
			expectedError: MatchError(ContainSubstring("found 2")),
		},
		"When no Secrets exist, it should return ErrUserDataSecretNotFound": {
			expectedError: MatchError(ErrUserDataSecretNotFound),
		},
		"When only Secrets of other NodeClasses exist, it should return ErrUserDataSecretNotFound": {
			objects:       []client.Object{newUserDataSecret("gpu", "7d0e4c6f")},
			expectedError: MatchError(ErrUserDataSecretNotFound),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.objects...).
				Build()

			secret, err := GetUserDataSecret(t.Context(), fakeClient, hcpNamespace, "default")

			if tc.expectedError != nil {
				g.Expect(err).To(tc.expectedError)
				g.Expect(secret).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Name).To(Equal(tc.expectedSecret))
			}
		})
	}
}

func TestKarpenterSecretPredicate(t *testing.T) {
	userDataSecret := newUserDataSecret("default", "3f1c9a2b")
	pullSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: hcpNamespace}}

	tests := map[string]struct {
		accepts func(p predicate.Predicate) bool
		want    bool
	}{
		"When a Karpenter Secret is created, it should accept the event": {
			accepts: func(p predicate.Predicate) bool { return p.Create(event.CreateEvent{Object: userDataSecret}) },
			want:    true,
		},
		"When a Karpenter Secret is updated, it should accept the event": {
			accepts: func(p predicate.Predicate) bool {
				return p.Update(event.UpdateEvent{ObjectOld: userDataSecret, ObjectNew: userDataSecret})
			},
			want: true,
		},
		"When another Secret is created, it should reject the event": {
			accepts: func(p predicate.Predicate) bool { return p.Create(event.CreateEvent{Object: pullSecret}) },
			want:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			NewWithT(t).Expect(tc.accepts(KarpenterSecretPredicate())).To(Equal(tc.want))
		})
	}
}

func TestMapUserDataSecretToNodeClass(t *testing.T) {
	tests := map[string]struct {
		secret *corev1.Secret
		want   []reconcile.Request
	}{
		"When a user data Secret changes, it should enqueue its NodeClass": {
			secret: newUserDataSecret("gpu", "7d0e4c6f"),
			want:   []reconcile.Request{{NamespacedName: client.ObjectKey{Name: "gpu"}}},
		},
		"When a token Secret changes, it should enqueue nothing": {
			secret: newTokenSecret("gpu", "7d0e4c6f"),
		},
		"When a Secret without NodePool annotation changes, it should enqueue nothing": {
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: hcpNamespace}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			NewWithT(t).Expect(mapUserDataSecretToNodeClass(t.Context(), tc.secret)).To(Equal(tc.want))
		})
	}
}

// newUserDataSecret returns the user data Secret that HyperShift generates for the NodeClass and config hash.
func newUserDataSecret(nodeClassName, configHash string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("user-data-%s-karpenter-%s", nodeClassName, configHash),
			Namespace:   hcpNamespace,
			Labels:      map[string]string{openshiftkarpenterv1.ManagedByKarpenterLabel: "true"},
			Annotations: map[string]string{openshiftkarpenterv1.TokenSecretNodePoolAnnotation: hcpNamespace + "/" + nodeClassName + "-karpenter"},
		},
	}
}

// newTokenSecret returns the token Secret that HyperShift generates for the NodeClass and config hash.
func newTokenSecret(nodeClassName, configHash string) *corev1.Secret {
	secret := newUserDataSecret(nodeClassName, configHash)
	secret.Name = fmt.Sprintf("token-%s-karpenter-%s", nodeClassName, configHash)
	secret.Annotations[tokenSecretAnnotation] = "true"
	return secret
}
