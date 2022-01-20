package resources

import (
	"github.com/abergmeier/terraform-provider-buildkit/pkg/buildctl/build"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func outputResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"type": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"dest": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"push": {
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
	}
}

func BuiltResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"addr": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"output": {
				Type: schema.TypeSet,
				Elem: outputResource(),
				Description: `Define exports for build result, e.g.
{
	type="image"
	name="docker.io/username/image"
	push=false
}`,
				MinItems:         1,
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

func validateOutput(output interface{}, p cty.Path) diag.Diagnostics {
	outputs := output.([]interface{})
	for _, outputi := range outputs {
		o := outputi.(*schema.ResourceData)
		t := o.Get("type").(string)
		d := o.Get("dest").(string)
		_, _, err := build.ResolveExporterDest(t, d)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}
