package provider

import (
	"context"
	"flag"
	"os"

	"github.com/abergmeier/terraform-provider-buildkit/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	tprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	tresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/moby/buildkit/client"
	bccommon "github.com/moby/buildkit/cmd/buildctl/common"
	"github.com/urfave/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	defaultConfigFlags = genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
	stderr             = os.Stderr
	schema             = tfsdk.Schema{
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
			"kubernetes": {
				Attributes:  tfsdk.SingleNestedAttributes(kubernetesAttributes),
				Description: "Special tooling for accessing Kubernetes",
				Optional:    true,
			},
		},
	}
)

func New() tprovider.Provider {
	return &provider{}
}

type provider struct {
	configured bool
}

type portForward struct {
	Service struct {
		name      string
		ports     []string
		namespace *string
	}
	Pod struct {
		name      string
		ports     []string
		namespace *string
	}
}

type data struct {
	addr          *string `tfsdk:"addr"`
	TlsServerName *string `tfsdk:"tlsservername"`
	kubernetes    struct {
		portForwards []portForward `tfsdk:"port_forwards"`
	} `tfsdk:"kubernetes"`
}

func (p *provider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {

	return schema, nil
}

func (p *provider) Configure(ctx context.Context, req tprovider.ConfigureRequest, resp *tprovider.ConfigureResponse) {
	d := data{}
	diags := req.Config.Get(ctx, &d)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, diags := toValidatedForwardOptions(d.kubernetes.portForwards)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Start forwarding ports
	for _, horriblerangebehaviorofgo := range opts {
		// TODO: Is there a way of making this end gracefully just before the program closes?
		go func(o *portforward.PortForwardOptions) {
			err := o.RunPortForward()
			if err != nil {
				tflog.Error(context.Background(), "Port forwarding failed", map[string]interface{}{"error": err})
			}
		}(horriblerangebehaviorofgo)
	}

	c, err := resolveClient(&d)
	if err != nil {
		resp.Diagnostics.AddError("Buildkit Client creation failed", err.Error())
		return
	}
	resp.ResourceData = c
}

func (p *provider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *provider) Resources(context.Context) []func() tresource.Resource {
	return []func() tresource.Resource{
		resources.NewBuiltResource,
	}
}

func toValidatedForwardOptions(portForwards []portForward) ([]*portforward.PortForwardOptions, diag.Diagnostics) {
	pfo := make([]*portforward.PortForwardOptions, 0, len(portForwards))

	for i, portForward := range portForwards {

		p := path.Root("kubernetes").AtName("port_forwards").AtListIndex(i)
		kubeConfigFlags := defaultConfigFlags
		matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
		f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
		o := newPortForwardOptions()
		if portForward.Service.name != "" {
			err := completeService(f, o, portForward.Service.name, portForward.Service.ports)
			if err != nil {
				return nil, diag.Diagnostics{
					diag.NewAttributeErrorDiagnostic(p, "Preparing local Service port forwarding failed", err.Error()),
				}
			}
		} else if portForward.Pod.name != "" {
			err := completePod(f, o, portForward.Pod.name, portForward.Pod.ports)
			if err != nil {
				return nil, diag.Diagnostics{
					diag.NewAttributeErrorDiagnostic(p, "Preparing local Pod port forwarding failed", err.Error()),
				}
			}
		} else {
			// TODO: Move this to validation
			return nil, diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(p, "Port forward invalid - neither Pod nor Service set", ""),
			}
		}

		err := o.Validate()
		if err != nil {
			return nil, diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(p, "Local port forwarding arguments not valid", err.Error()),
			}
		}
		pfo = append(pfo, o)
	}
	return pfo, nil
}

func resolveClient(d *data) (*client.Client, error) {

	flagSet := flag.NewFlagSet("buildkit", flag.ContinueOnError)
	flagSet.Set("tlsservername", *d.TlsServerName)
	flagSet.Set("addr", *d.addr)
	return bccommon.ResolveClient(cli.NewContext(nil, &flag.FlagSet{}, nil))
}
