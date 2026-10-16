package operator

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/openshift/karpenter-operator/pkg/cloudprovider/common"
	testfake "github.com/openshift/karpenter-operator/test/pkg/fake"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSchemeIncludesKarpenterTypes(t *testing.T) {
	for _, kind := range []string{"NodeClaim", "NodeClaimList"} {
		t.Run(kind, func(t *testing.T) {
			gvk := schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: kind}
			if !scheme.Recognizes(gvk) {
				t.Errorf("scheme does not recognize %s", gvk)
			}
		})
	}
}

func TestLoadEnv(t *testing.T) {
	t.Setenv(ReleaseVersionEnvName, "4.23.0")
	t.Setenv(ClusterNameEnvName, "my-cluster")
	t.Setenv(ClusterEndpointEnvName, "https://api-int.example.com:6443")
	t.Setenv(PlatformEnvName, "AWS")
	t.Setenv(RegionEnvName, "us-east-1")
	t.Setenv(TokenMinterImageEnvName, "quay.io/openshift/hypershift:latest")

	tests := map[string]struct {
		managementCluster      string
		securityContextUID     string
		wantManagementCluster  bool
		wantSecurityContextUID *int64
		wantErr                string
	}{
		"When all environment variables are set, it should load the configured options": {
			managementCluster:      "true",
			securityContextUID:     "1002",
			wantManagementCluster:  true,
			wantSecurityContextUID: new(int64(1002)),
		},
		"When management cluster mode is empty, it should default to standalone mode": {
			wantManagementCluster:  false,
			wantSecurityContextUID: nil,
		},
		"When management cluster mode is false, it should load standalone mode": {
			managementCluster:     "false",
			wantManagementCluster: false,
		},
		"When management cluster mode is not a boolean, it should return an error": {
			managementCluster: "notabool",
			wantErr:           "invalid MANAGEMENT_CLUSTER",
		},
		"When the UID is empty, it should leave the UID unspecified": {
			managementCluster:      "true",
			wantManagementCluster:  true,
			wantSecurityContextUID: nil,
		},
		"When the UID is root, it should load it for validation": {
			managementCluster:      "true",
			securityContextUID:     "0",
			wantManagementCluster:  true,
			wantSecurityContextUID: new(int64(0)),
		},
		"When the UID is negative, it should load it for validation": {
			managementCluster:      "true",
			securityContextUID:     "-1",
			wantManagementCluster:  true,
			wantSecurityContextUID: new(int64(-1)),
		},
		"When the UID is not an integer, it should return an error": {
			managementCluster:      "true",
			securityContextUID:     "not-a-uid",
			wantSecurityContextUID: nil,
			wantErr:                "loading DEFAULT_SECURITY_CONTEXT_UID",
		},
		"When the UID overflows, it should return an error": {
			managementCluster:      "true",
			securityContextUID:     "9223372036854775808",
			wantSecurityContextUID: nil,
			wantErr:                "loading DEFAULT_SECURITY_CONTEXT_UID",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			t.Setenv(ManagementClusterEnvName, tc.managementCluster)
			t.Setenv(DefaultSecurityContextUIDEnvName, tc.securityContextUID)

			var opts Options
			err := opts.LoadEnv()
			if tc.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.wantErr)))
				g.Expect(opts.DefaultSecurityContextUID).To(Equal(tc.wantSecurityContextUID))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts).To(Equal(Options{
				ReleaseVersion:            "4.23.0",
				ClusterName:               "my-cluster",
				ClusterEndpoint:           "https://api-int.example.com:6443",
				Platform:                  "AWS",
				Region:                    "us-east-1",
				ManagementCluster:         tc.wantManagementCluster,
				TokenMinterImage:          "quay.io/openshift/hypershift:latest",
				DefaultSecurityContextUID: tc.wantSecurityContextUID,
			}))
		})
	}
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		opts    Options
		wantErr string
	}{
		"When required fields are set, it should accept the options": {
			opts: Options{
				Namespace:      "openshift-karpenter",
				ReleaseVersion: "4.23.0",
			},
			wantErr: "",
		},
		"When the namespace is missing, it should reject the options": {
			opts: Options{
				ReleaseVersion: "4.23.0",
			},
			wantErr: "required configuration not set: --namespace",
		},
		"When all options are empty, it should report the missing namespace": {
			opts:    Options{},
			wantErr: "required configuration not set: --namespace",
		},
		"When optional environment variables are empty, it should accept the options": {
			opts: Options{
				Namespace: "openshift-karpenter",
			},
			wantErr: "",
		},
		"When management cluster fields are missing, it should reject the options": {
			opts: Options{
				Namespace:         "openshift-karpenter",
				ManagementCluster: true,
			},
			wantErr: "required configuration not set: --target-kubeconfig, CLUSTER_NAME, CLUSTER_ENDPOINT, PLATFORM, REGION, TOKEN_MINTER_IMAGE",
		},
		"When management cluster fields are set, it should accept the options": {
			opts: Options{
				Namespace:         "openshift-karpenter",
				ManagementCluster: true,
				TargetKubeconfig:  "/var/run/secrets/kubeconfig",
				ClusterName:       "my-cluster",
				ClusterEndpoint:   "https://api-int.example.com:6443",
				Platform:          "AWS",
				Region:            "us-east-1",
				TokenMinterImage:  "quay.io/openshift/hypershift:latest",
			},
			wantErr: "",
		},
		"When the region and token-minter image are missing, it should report both management cluster fields": {
			opts: Options{
				Namespace:         "openshift-karpenter",
				ManagementCluster: true,
				TargetKubeconfig:  "/var/run/secrets/kubeconfig",
				ClusterName:       "my-cluster",
				ClusterEndpoint:   "https://api-int.example.com:6443",
				Platform:          "AWS",
			},
			wantErr: "required configuration not set: REGION, TOKEN_MINTER_IMAGE",
		},
		"When the UID is unset, it should accept the options": {
			opts: Options{
				Namespace:                 "clusters-autoscale",
				DefaultSecurityContextUID: nil,
			},
			wantErr: "",
		},
		"When the UID is positive, it should accept the options": {
			opts: Options{
				Namespace:                 "clusters-autoscale",
				DefaultSecurityContextUID: new(int64(1002)),
			},
			wantErr: "",
		},
		"When the UID is root, it should reject the options": {
			opts: Options{
				Namespace:                 "clusters-autoscale",
				DefaultSecurityContextUID: new(int64(0)),
			},
			wantErr: "DEFAULT_SECURITY_CONTEXT_UID must be a positive non-root UID",
		},
		"When the UID is negative, it should reject the options": {
			opts: Options{
				Namespace:                 "clusters-autoscale",
				DefaultSecurityContextUID: new(int64(-1)),
			},
			wantErr: "DEFAULT_SECURITY_CONTEXT_UID must be a positive non-root UID",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			err := tc.opts.Validate()
			if tc.wantErr != "" {
				g.Expect(err).To(MatchError(tc.wantErr))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestResolveControllerConfig(t *testing.T) {
	tests := map[string]struct {
		uid     *int64
		wantUID *int64
	}{
		"When the UID is unset, it should leave the controller UID unspecified": {
			wantUID: nil,
		},
		"When a UID is supplied, it should pass it to the controller configuration": {
			uid:     new(int64(1002)),
			wantUID: new(int64(1002)),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			opts := Options{DefaultSecurityContextUID: tc.uid}
			cfg := opts.ResolveControllerConfig(common.InfrastructureInfo{}, &testfake.CloudProvider{})
			g.Expect(cfg.DefaultSecurityContextUID).To(Equal(tc.wantUID))
		})
	}
}
