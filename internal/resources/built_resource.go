package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/abergmeier/terraform-provider-buildkit/pkg/buildctl"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/util/entitlements"
)

var (
	builtSchema = tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"output": {
				Type:                types.StringType,
				MarkdownDescription: "Define exports for build result, e.g. `type=image,name=docker.io/username/image,push=true`",
				Optional:            true,
			},
			"trace": {
				Type:        types.StringType,
				Description: "Path to trace file. Defaults to no tracing.",
				Optional:    true,
			},
			"local_dirs": {
				Type: types.MapType{
					ElemType: types.StringType,
				},
				Description: "Allow build access to the local directory",
				Optional:    true,
			},
			"oci_layout": {
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Description: "Allow build access to the local OCI layout",
				Optional:    true,
			},
			"frontend": {
				Type:        types.StringType,
				Description: "Define frontend used for build",
				Optional:    true,
			},
			"opts": {
				Type: types.MapType{
					ElemType: types.StringType,
				},
				MarkdownDescription: "Define custom options for frontend, e.g. `{target=\"foo\", \"build-arg:foo\"=\"bar\"}`",
				Optional:            true,
			},
			"cache": {
				Attributes: tfsdk.SingleNestedAttributes(
					map[string]tfsdk.Attribute{
						"disable": {
							Type:        types.BoolType,
							Description: "Disable cache for all the vertices",
							Optional:    true,
						},
						"export": {
							Attributes: tfsdk.SingleNestedAttributes(
								map[string]tfsdk.Attribute{
									"strings": {
										Type: types.ListType{
											ElemType: types.StringType,
										},
										MarkdownDescription: "Export build cache, e.g. [\"type=registry,ref=example.com/foo/bar\"], or [\"type=local,dest=path/to/dir\"]",
										Optional:            true,
									},
									"opts": {
										Type: types.ListType{
											ElemType: types.StringType,
										},
										Optional: true,
									},
								},
							),
						},
						"import": {
							Type: types.ListType{
								ElemType: types.StringType,
							},
							MarkdownDescription: "Import build cache, e.g. [\"type=registry,ref=example.com/foo/bar\"], or [\"type=local,src=path/to/dir\"]",
							Optional:            true,
						},
					},
				),
			},
			"secret": {
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Description: "Secret value exposed to the build. Format id=secretname,src=filepath",
				Optional:    true,
			},
			"allow": {
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Description: "Allow extra privileged entitlement, e.g. network.host, security.insecure",
				Optional:    true,
			},
			"metadata_file": {
				Type:        types.StringType,
				Description: "Output build metadata (e.g., image digest) to a file as JSON",
				Optional:    true,
			},
		},
	}
)

type builtResource struct {
}

func NewBuiltResource() tresource.Resource {
	return &builtResource{}
}

func (r *builtResource) Metadata(context.Context, tresource.MetadataRequest, *tresource.MetadataResponse) {

}

func (r *builtResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return builtSchema, nil
}

type builtConfig struct {
	Allow []string `tfsdk:"allow"`
	Cache struct {
		Disable       bool     `tfsdk:"disable"`
		ExportStrings []string `tfsdk:"export_strings"`
		ImportStrings []string `tfsdk:"import_strings"`
	} `tfsdk:"cache"`
	Opts          map[string]string  `tfsdk:"opts"`
	LocalDirs     map[string]string  `tfsdk:"local_dirs"`
	OutputStrings []string           `tfsdk:"output_strings"`
	Secrets       []secretAttachment `tfsdk:"secrets"`
}

type secretAttachment struct {
	ID       string `tfsdk:"id"`
	FilePath string `tfsdk:"file_path"`
	Env      string `tfsdk:"env"`
}

func (r *builtResource) Create(ctx context.Context, req tresource.CreateRequest, resp *tresource.CreateResponse) {

	var c *client.Client
	diags := req.ProviderMeta.Get(ctx, &c)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := builtConfig{}
	diags = req.Plan.Get(ctx, &cfg)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ent, diags := parseAllow(cfg.Allow)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, diags := parseSecrets(cfg.Secrets)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	exports, diags := parseOutput(cfg.OutputStrings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cacheExports, diags := parseExportCache(cfg.Cache.ExportStrings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cacheImports, diags := parseImportCache(cfg.Cache.ImportStrings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	bc := buildctl.BuildConfig{
		AllowedEntitlements: ent,
		SecretAttachables:   sa,
		Exports:             exports,
		ExportCaches:        cacheExports,
		ImportCaches:        cacheImports,
		FrontendAttrs:       parseOpts(cfg.Opts),
		LocalDirs:           parseLocal(cfg.LocalDirs),
	}

	err := buildctl.BuildAction(ctx, c, &bc)
	if err != nil {
		resp.Diagnostics.AddError("Building failed", err.Error())
		return
	}
}

func (r *builtResource) Read(context.Context, tresource.ReadRequest, *tresource.ReadResponse) {

}

func (r *builtResource) Update(context.Context, tresource.UpdateRequest, *tresource.UpdateResponse) {

}

func (r *builtResource) Delete(context.Context, tresource.DeleteRequest, *tresource.DeleteResponse) {

}

func parseAllow(inp []string) ([]entitlements.Entitlement, diag.Diagnostics) {
	ent := make([]entitlements.Entitlement, 0, len(inp))
	for i, v := range inp {
		e, err := entitlements.Parse(v)
		if err != nil {
			return nil, diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(path.Root("allow").AtListIndex(i), "Parsing Entitlement failed", err.Error()),
			}
		}
		ent = append(ent, e)
	}
	return ent, nil
}

func parseExportCache(exportCaches []string) ([]client.CacheOptionsEntry, diag.Diagnostics) {
	cacheExports, err := build.ParseExportCache(exportCaches, nil)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewAttributeErrorDiagnostic(path.Root("cache").AtName("export_strings"), "Parsing one Export Cache failed", err.Error()),
		}
	}

	return cacheExports, nil
}

func parseImportCache(importCaches []string) ([]client.CacheOptionsEntry, diag.Diagnostics) {
	cacheImports, err := build.ParseImportCache(importCaches)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewAttributeErrorDiagnostic(path.Root("cache").AtName("import_strings"), "Parse one Import Cache failed", err.Error()),
		}
	}

	return cacheImports, nil
}

func parseLocal(locals map[string]string) map[string]string {
	return locals
}

func parseSecrets(sl []secretAttachment) (session.Attachable, diag.Diagnostics) {
	fs := make([]secretsprovider.Source, 0, len(sl))
	for _, v := range sl {
		fs = append(fs, secretsprovider.Source{
			ID:       v.ID,
			FilePath: v.FilePath,
			Env:      v.Env,
		})
	}
	store, err := secretsprovider.NewStore(fs)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewAttributeErrorDiagnostic(path.Root("secrets"), "Creating secrets Store failed", err.Error()),
		}
	}
	return secretsprovider.NewSecretProvider(store), nil
}

func parseOpts(opts map[string]string) map[string]string {
	return opts
}

func parseOutput(exports []string) ([]client.ExportEntry, diag.Diagnostics) {
	// TODO: Replace parsing by moving these structures to Terraform type system
	out, err := build.ParseOutput(exports)
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewAttributeErrorDiagnostic(path.Root("outputs"), "Parsing one output failed", err.Error()),
		}
	}
	return out, nil
}
