package resources

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/util/progress/progresswriter"
)

func createBuilt(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	ois := d.Get("output").([]interface{})

	addr := d.Get("addr").(string)

	outputMaps := make([]map[string]interface{}, len(ois))
	for i, oi := range ois {
		outputMaps[i] = oi.(map[string]interface{})
	}

	outputStrings := make([]string, len(ois))
	for i, om := range outputMaps {
		outputStrings[i] = fmt.Sprintf("type=%s,name=%s,push=%s", om["type"].(string), om["name"].(string), om["push"].(bool))
	}

	exports, err := build.ParseOutput(outputStrings)
	if err != nil {
		return diag.FromErr(err)
	}

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
	fmt.Sprintf("RESP %#v", resp)
	return nil
}
