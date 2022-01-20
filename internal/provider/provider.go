package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/abergmeier/terraform-provider-buildkit/internal/archive"
	"github.com/abergmeier/terraform-provider-buildkit/internal/resources"
	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	provider := &schema.Provider{
		/*DataSourcesMap: map[string]*schema.Resource{
			"buildkit_built": datasource.BuiltResource(),
		},*/
		ResourcesMap: map[string]*schema.Resource{
			"buildkit_built": resources.BuiltResource(),
		},
		Schema: map[string]*schema.Schema{
			"addr": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: defaultAddr,
			},
			"bootstrap": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"helper": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "rootlesskit",
						},
						"binary": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "buildkitd",
						},
						"flags": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"release": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "v0.9.3",
						},
					},
				},
				MaxItems: 1,
			},
		},
	}
	provider.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		// Shameless plug from https://github.com/terraform-providers/terraform-provider-aws/blob/d51784148586f605ab30ecea268e80fe83d415a9/aws/provider.go
		terraformVersion := provider.TerraformVersion
		if terraformVersion == "" {
			// Terraform 0.12 introduced this field to the protocol
			// We can therefore assume that if it's missing it's 0.10 or 0.11
			terraformVersion = "0.11+compatible"
		}
		return providerConfigure(ctx, d, terraformVersion)
	}
	return provider
}

func defaultAddr() (interface{}, error) {
	xrd := os.Getenv("XDG_RUNTIME_DIR")
	if xrd == "" {
		return nil, errors.New("no Environment XDG_RUNTIME_DIR found")
	}
	return fmt.Sprintf("unix://%s/buildkit/buildkitd.sock", xrd), nil
}

func cacheDir() string {
	xch := os.Getenv("XDG_CACHE_HOME")
	if xch != "" {
		return xch
	}

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s/.cache", home)
}

func providerConfigure(ctx context.Context, d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {
	addr := d.Get("addr").(string)
	release := d.Get("release").(string)
	bootstrap := d.Get("bootstrap").([]interface{})
	if len(bootstrap) == 0 {
		// assume buildkitd is already running
		err := waitOnBuildkitd(ctx, addr)
		return nil, diag.FromErr(err)
	}

	//downloadUrl := fmt.Sprintf("https://github.com/moby/buildkit/releases/download/%s/buildkit-%s.linux-amd64.tar.gz", release, release)

	destPath := fmt.Sprintf("%s/terraform_provider_buildkit/buildkit-%s.linux-amd64", cacheDir(), release)
	localPath := fmt.Sprintf("%s/terraform_provider_buildkit/buildkit-%s.linux-amd64.tar.gz", cacheDir(), release)
	tgz, err := os.Open(localPath)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	defer tgz.Close()
	err = archive.Extract(tgz, destPath)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	// We need to bootstrap
	bd := bootstrap[0].(*schema.ResourceData)
	helper := bd.Get("helper").(string)
	binary := bd.Get("binary").(string)
	var bp *exec.Cmd
	if helper == "" {
		bp = exec.Command(binary, "--addr", addr)
	} else {
		bp = exec.CommandContext(ctx, helper, binary, "--addr", addr)
	}

	SockAddr := "/tmp/echo.sock"

	err = os.RemoveAll(SockAddr)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	l, err := net.Listen("unix", SockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer l.Close()

	bp.Env = append(bp.Env, "NOTIFY_SOCKET=")

	err = bp.Start()
	if err != nil {
		return nil, diag.FromErr(err)
	}

	err = waitOnBuildkitd(ctx, l)
	return nil, diag.FromErr(err)
}

func waitOnBuildkitd(ctx context.Context, l net.Listener) error {
	buf := make([]byte, len([]byte(daemon.SdNotifyReady)))
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		_, err = conn.Read(buf)
		if err != nil {
			return err
		}

		if string(buf) != daemon.SdNotifyReady {
			panic(buf)
		}

		break
	}
	return nil
}
