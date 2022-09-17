package main

import (
	"context"

	"github.com/abergmeier/terraform-provider-buildkit/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "hashicorp.com/abergmeier/buildkit",
	})
}
