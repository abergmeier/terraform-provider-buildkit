package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestCreateBuilt(t *testing.T) {
	d := BuiltResource().TestResourceData()
	out := outputResource().TestResourceData()
	setResourceData(t, out, "type", "foo")
	setResourceData(t, d, "output", []interface{}{
		out,
	})
	createBuilt(context.Background(), d, nil)
}

func setResourceData(t *testing.T, d *schema.ResourceData, key string, value interface{}) {
	err := d.Set(key, value)
	if err != nil {
		t.Fatal(err)
	}
}
