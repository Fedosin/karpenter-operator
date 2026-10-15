package hypershift

import (
	"context"
	"errors"
	"fmt"
	"strings"

	openshiftkarpenterv1 "github.com/openshift/karpenter-operator/api/karpenter/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// tokenSecretAnnotation marks HyperShift NodePool token Secrets, as opposed to user data Secrets.
	tokenSecretAnnotation = "hypershift.openshift.io/ignition-config"

	// karpenterNodePoolSuffix is appended to the NodeClass name for its in-memory HyperShift NodePool.
	karpenterNodePoolSuffix = "-karpenter"
)

// karpenterSecretSelector matches the Karpenter token and user data Secrets created by HyperShift.
var karpenterSecretSelector = labels.SelectorFromSet(labels.Set{openshiftkarpenterv1.ManagedByKarpenterLabel: "true"})

// ErrUserDataSecretNotFound is returned when the namespace contains no user data Secret for the NodeClass.
var ErrUserDataSecretNotFound = errors.New("user data secret not found")

// GetUserDataSecret returns the user data Secret that HyperShift generated for the NodeClass in the given namespace.
// HyperShift deletes the outdated user data Secret before it creates the one of a new config, so several matching
// Secrets are unexpected and return an error instead of a guess.
func GetUserDataSecret(ctx context.Context, c client.Reader, namespace, nodeClassName string) (*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: karpenterSecretSelector}); err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}

	expectedNodePool := client.ObjectKey{Namespace: namespace, Name: karpenterNodePoolName(nodeClassName)}.String()
	var userDataSecrets []corev1.Secret
	for _, secret := range secretList.Items {
		nodePool := secret.Annotations[openshiftkarpenterv1.TokenSecretNodePoolAnnotation]
		if nodePool == expectedNodePool && secret.Annotations[tokenSecretAnnotation] != "true" {
			userDataSecrets = append(userDataSecrets, secret)
		}
	}

	switch len(userDataSecrets) {
	case 0:
		return nil, fmt.Errorf("%w for NodeClass %q", ErrUserDataSecretNotFound, nodeClassName)
	case 1:
		return &userDataSecrets[0], nil
	default:
		return nil, fmt.Errorf("expected one user data secret for NodeClass %q, found %d", nodeClassName, len(userDataSecrets))
	}
}

// karpenterNodePoolName returns the name of the in-memory HyperShift NodePool generated for the NodeClass.
// See https://github.com/openshift/hypershift/blob/e4c842ae96f6b325a148fad1dab76d45a73a4858/support/karpenter/karpenter.go#L131-L134
func karpenterNodePoolName(nodeClassName string) string {
	return nodeClassName + karpenterNodePoolSuffix
}

// KarpenterSecretPredicate only returns true on creates/updates of Secrets matching karpenterSecretSelector.
func KarpenterSecretPredicate() predicate.Predicate {
	isKarpenterSecret := func(obj client.Object) bool {
		return karpenterSecretSelector.Matches(labels.Set(obj.GetLabels()))
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isKarpenterSecret(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isKarpenterSecret(e.ObjectNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

// EnqueueNodeClassForUserDataSecret enqueues the NodeClass that a user data Secret belongs to.
func EnqueueNodeClassForUserDataSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(mapUserDataSecretToNodeClass)
}

// mapUserDataSecretToNodeClass maps a user data Secret to its NodeClass. Token Secrets are ignored because the
// NodeClass controllers only read the user data Secret.
func mapUserDataSecretToNodeClass(_ context.Context, obj client.Object) []reconcile.Request {
	if obj.GetAnnotations()[tokenSecretAnnotation] == "true" {
		return nil
	}
	_, nodePoolName, _ := strings.Cut(obj.GetAnnotations()[openshiftkarpenterv1.TokenSecretNodePoolAnnotation], "/")
	nodeClassName, ok := strings.CutSuffix(nodePoolName, karpenterNodePoolSuffix)
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: nodeClassName}}}
}
