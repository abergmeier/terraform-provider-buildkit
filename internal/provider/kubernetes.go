package provider

import (
	"github.com/abergmeier/terraform-provider-buildkit/internal/planmodifiers"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	kubernetesAttributes = map[string]tfsdk.Attribute{
		"host": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_HOST"),
			},
			Description: "The hostname (in form of URI) of Kubernetes master.",
		},
		"username": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_USER"),
			},
			Description: "The username to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
		},
		"password": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_PASSWORD"),
			},
			Description: "The password to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
		},
		"insecure": {
			Type:     types.BoolType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_INSECURE"),
			},
			Description: "Whether server should be accessed without verifying the TLS certificate.",
		},
		"client_certificate": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CLIENT_CERT_DATA"),
			},
			Description: "PEM-encoded client certificate for TLS authentication.",
		},
		"client_key": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CLIENT_KEY_DATA"),
			},
			Description: "PEM-encoded client certificate key for TLS authentication.",
		},
		"cluster_ca_certificate": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CLUSTER_CA_CERT_DATA"),
			},
			Description: "PEM-encoded root certificates bundle for TLS authentication.",
		},
		"config_paths": {
			Type: types.ListType{
				ElemType: types.StringType,
			},
			Optional:            true,
			MarkdownDescription: "A list of paths to kube config files. Can be set with `KUBE_CONFIG_PATHS` environment variable.",
		},
		"config_context": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CTX"),
			},
		},
		"config_context_auth_info": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CTX_AUTH_INFO"),
			},
			Description: "",
		},
		"config_context_cluster": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_CTX_CLUSTER"),
			},
		},
		"token": {
			Type:     types.StringType,
			Optional: true,
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_TOKEN"),
			},
			Description: "Token to authenticate an service account",
		},
		"proxy_url": {
			Type:        types.StringType,
			Optional:    true,
			Description: "URL to the proxy to be used for all API requests",
			PlanModifiers: tfsdk.AttributePlanModifiers{
				planmodifiers.FallbackToEnvStringModifier("KUBE_PROXY_URL"),
			},
		},
		"port_forwards": {
			Description: "Port forwardings which will be active while provider is in use",
			Attributes: tfsdk.ListNestedAttributes(
				map[string]tfsdk.Attribute{"service": {
					Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
						"name": {
							Type:     types.StringType,
							Required: true,
						},
						"namespace": {
							Type:     types.StringType,
							Optional: true,
						},
						"ports": {
							Type:     types.StringType,
							Required: true,
						},
					}),
				}, "pod": {
					Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
						"name": {
							Type:     types.StringType,
							Required: true,
						},
						"namespace": {
							Type:     types.StringType,
							Optional: true,
						},
						"ports": {
							Type:     types.StringType,
							Required: true,
						},
					}),
				}},
			),
		},
	}
)
