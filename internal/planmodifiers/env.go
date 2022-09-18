package planmodifiers

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func FallbackToEnvStringModifier(key string) tfsdk.AttributePlanModifier {
	return &fallbackToEnvString{
		key: key,
	}
}

type fallbackToEnvString struct {
	key string
}

func (m *fallbackToEnvString) Description(context.Context) string {
	return "Falls back to Environment Variable if no string was provided"
}

func (m *fallbackToEnvString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *fallbackToEnvString) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	v := types.String{}
	diags := tfsdk.ValueAs(ctx, req.AttributePlan, &v)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !v.IsNull() {
		return
	}

	envValue, ok := os.LookupEnv(m.key)
	if !ok {
		return
	}

	resp.AttributePlan = types.String{
		Value: envValue,
	}
}
