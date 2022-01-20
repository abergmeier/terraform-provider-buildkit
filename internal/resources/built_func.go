package resources

import (
	"context"
	"fmt"
	"os"

	"github.com/abergmeier/terraform-provider-buildkit/pkg/buildctl/build"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/progress/progresswriter"
)

func createBuilt(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	outputs := d.Get("output").([]interface{})

	exports := make([]client.ExportEntry, len(outputs))

	err := error(nil)
	for i, outputi := range outputs {
		exports[i].Attrs = map[string]string{}
		o := outputi.(*schema.ResourceData)
		exports[i].Type = o.Get("type").(string)
		d := o.Get("dest").(string)
		exports[i].Output, exports[i].OutputDir, err = build.ResolveExporterDest(exports[i].Type, d)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	addr := d.Get("addr").(string)

	return createBuiltWithSolveOpt(ctx, client.SolveOpt{
		Exports: exports,
	}, addr)
}

func deleteBuilt(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
	return nil
}

func updateBuilt(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
	return nil
}

func readBuilt(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
	return nil
}

func createBuiltWithSolveOpt(ctx context.Context, solveOpt client.SolveOpt, addr string) diag.Diagnostics {
	c, err := client.New(ctx, addr)
	if err != nil {
		return diag.FromErr(err)
	}

	var def *llb.Definition
	// not using shared context to not disrupt display but let is finish reporting errors
	pw, err := progresswriter.NewPrinter(context.TODO(), os.Stderr, "plain")
	if err != nil {
		return diag.FromErr(err)
	}
	mw := progresswriter.NewMultiWriter(pw)
	resp, err := c.Solve(ctx, def, solveOpt, progresswriter.ResetTime(mw.WithPrefix("DEBUG", false)).Status())
	if err != nil {
		return diag.FromErr(err)
	}
	fmt.Printf("RESP %#v", resp)
	return nil
}
