package provider

import (
	"context"
	"os"

	"github.com/abergmeier/terraform-provider-buildkit/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	tresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/kubectl/pkg/cmd/portforward"
)

var stderr = os.Stderr

func New() tprovider.Provider {
	return &provider{}
}

type provider struct {
	configured bool
}

type data struct {
	addr                  *string
	kubernetesPortForward struct {
		Service struct {
			name      string
			ports     string
			namespace *string
		}
		Pod struct {
			name      string
			ports     string
			namespace *string
		}
	}
}

func (p *provider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {

	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"addr": {
				Type:                types.StringType,
				MarkdownDescription: "buildkitd address (default: `unix:///run/buildkit/buildkitd.sock`)",
				Optional:            true,
			},
			"debug": {
				Type:        types.BoolType,
				Description: "enable debug output in logs",
				Optional:    true,
			},
			"tlsservername": {
				Type:        types.StringType,
				Description: "buildkitd server name for certificate validation",
				Optional:    true,
			},
			"tlscacert": {
				Type:        types.StringType,
				Description: "CA certificate for validation",
				Optional:    true,
			},
			"tlscert": {
				Type:        types.StringType,
				Description: "client certificate",
				Optional:    true,
			},
			"tlskey": {
				Type:        types.StringType,
				Description: "client key",
				Optional:    true,
			},
			"tlsdir": {
				Type:        types.StringType,
				Description: "directory containing CA certificate, client certificate, and client key",
				Optional:    true,
			},
			"timeout": {
				Type:                types.Int64Type,
				MarkdownDescription: "timeout backend connection after value seconds (default: `5`)",
				Optional:            true,
			},
			"kubernetes_port_forward": {
				Attributes: tfsdk.MapNestedAttributes(
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
		},
	}, nil
}

func (p *provider) Configure(ctx context.Context, req tprovider.ConfigureRequest, resp *tprovider.ConfigureResponse) {
	d := data{}
	diags := req.Config.Get(ctx, &d)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var o *portforward.PortForwardOptions
	if d.kubernetesPortForward.Service.name != "" {
		o = newPortForwardOptions()
		err := completeService(o, d.kubernetesPortForward.Service.name)
		if err != nil {
			resp.Diagnostics.AddAttributeError("kubernetes_port_forward", "Preparing local Service port forwarding failed", err.Error())
			return
		}
	} else if d.kubernetesPortForward.Pod.name != "" {
		o = newPortForwardOptions()
		err := completePod(o, d.kubernetesPortForward.Pod.name)
		if err != nil {
			resp.Diagnostics.AddAttributeError("kubernetes_port_forward", "Preparing local Pod port forwarding failed", err.Error())
			return
		}
	}

	err := o.Validate()
	if err != nil {
		resp.Diagnostics.AddAttributeError("kubernetes_port_forward", "Local port forwarding arguments not valid", err.Error())
		return
	}

	// TODO: Is there a way of making this end gracefully just before the program closes?
	go func() {
		err := o.RunPortForward()
		if err != nil {
			return err
		}
	}()
}

func (p *provider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *provider) Resources(context.Context) []func() tresource.Resource {
	return []func() tresource.Resource{
		resources.NewBuiltResource,
	}
}
