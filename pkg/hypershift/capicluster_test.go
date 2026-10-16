package hypershift

import (
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveCAPIClusterGVK(t *testing.T) {
	v1beta1 := schema.GroupVersion{Group: "cluster.x-k8s.io", Version: "v1beta1"}
	v1beta2 := schema.GroupVersion{Group: "cluster.x-k8s.io", Version: "v1beta2"}

	tests := map[string]struct {
		servedVersions []schema.GroupVersion
		wantGVK        schema.GroupVersionKind
		wantError      bool
	}{
		"When multiple versions are served, it should return the preferred version": {
			// The first version is preferred.
			servedVersions: []schema.GroupVersion{v1beta2, v1beta1},
			wantGVK:        v1beta2.WithKind("Cluster"),
		},
		"When only an older version is served, it should return that version": {
			servedVersions: []schema.GroupVersion{v1beta1},
			wantGVK:        v1beta1.WithKind("Cluster"),
		},
		"When the CAPI Cluster is not served, it should return an error": {
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			mapper := meta.NewDefaultRESTMapper(tc.servedVersions)
			for _, gv := range tc.servedVersions {
				mapper.Add(gv.WithKind("Cluster"), meta.RESTScopeNamespace)
			}

			gvk, err := ResolveCAPIClusterGVK(mapper)
			if tc.wantError {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(gvk).To(Equal(tc.wantGVK))
		})
	}
}
