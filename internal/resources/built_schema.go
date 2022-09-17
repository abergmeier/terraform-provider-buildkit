package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	tresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/moby/buildkit/cmd/buildctl/build"
)

type builtResource struct {
}

func NewBuiltResource() tresource.Resource {
	return &builtResource{}
}

func (r *builtResource) Metadata(context.Context, tresource.MetadataRequest, *tresource.MetadataResponse) {

}

func (r *builtResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {

}

func (r *builtResource) Create(context.Context, tresource.CreateRequest, *tresource.CreateResponse) {

}

func (r *builtResource) Read(context.Context, tresource.ReadRequest, *tresource.ReadResponse) {

}

func (r *builtResource) Update(context.Context, tresource.UpdateRequest, *tresource.UpdateResponse) {

}

func (r *builtResource) Delete(context.Context, tresource.DeleteRequest, *tresource.DeleteResponse) {

}

func BuiltResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"addr": {
				Type:     schema.TypeString,
				Required: true,
			},
			"output": {
				Description: `Define exports for build result, e.g.
{
	type = image
	name=docker.io/username/image
	push=false`,
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
				},
				Required:         true,
				ValidateDiagFunc: validateOutput,
			},
		},
		Description: `Copy an image (manifest, filesystem layers, signatures) from one location to another.
Uses the system's trust policy to validate images, rejects images not trusted by the policy.
source-image and destination-image are interpreted completely independently; e.g. the destination name does not automatically inherit any parts of the source name.`,
		CreateContext: createBuilt,
		ReadContext:   readBuilt,
		UpdateContext: updateBuilt,
		DeleteContext: deleteBuilt,
	}
}

func validateOutput(v interface{}, p cty.Path) diag.Diagnostics {
	vs := v.([]interface{})
	for _, vi := range vs {
		vm := vi.(map[string]interface{})
		_, err := build.ParseOutput([]string{fmt.Sprintf("type=%s,name=%s,push=%s", vm["type"].(string), vm["name"].(string), vm["push"].(bool))})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}
