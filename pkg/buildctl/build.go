package buildctl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"

	"github.com/containerd/continuity"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type BuildConfig struct {
	AllowedEntitlements []entitlements.Entitlement
	ExportCaches        []client.CacheOptionsEntry
	Frontend            string
	FrontendAttrs       map[string]string
	ImportCaches        []client.CacheOptionsEntry
	LocalDirs           map[string]string
	MetadataFile        string
	NoCache             bool
	Exports             []client.ExportEntry
	ProgressMode        string
	TracefileName       string
	SecretAttachables   session.Attachable
}

func read(r io.Reader, cfg *BuildConfig) (*llb.Definition, error) {
	def, err := llb.ReadFrom(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse input")
	}
	if cfg.NoCache {
		for _, dt := range def.Def {
			var op pb.Op
			if err := (&op).Unmarshal(dt); err != nil {
				return nil, errors.Wrap(err, "failed to parse llb proto op")
			}
			dgst := digest.FromBytes(dt)
			opMetadata, ok := def.Metadata[dgst]
			if !ok {
				opMetadata = pb.OpMetadata{}
			}
			c := llb.Constraints{Metadata: opMetadata}
			llb.IgnoreCache(&c)
			def.Metadata[dgst] = c.Metadata
		}
	}
	return def, nil
}

func openTraceFile(cfg *BuildConfig) (*os.File, error) {
	if traceFileName := cfg.TracefileName; traceFileName != "" {
		return os.OpenFile(traceFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	}
	return nil, nil
}

// BuildAction implements building based on Buildkit code.
// Most parsing does however already happen outside of BuildAction
func BuildAction(ctx context.Context, c *client.Client, cfg *BuildConfig) error {

	traceFile, err := openTraceFile(cfg)
	if err != nil {
		return err
	}
	var traceEnc *json.Encoder
	if traceFile != nil {
		defer traceFile.Close()
		traceEnc = json.NewEncoder(traceFile)

		logrus.Infof("tracing logs to %s", traceFile.Name())
	}

	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}
	attachable = append(attachable, cfg.SecretAttachables)

	allowed := cfg.AllowedEntitlements

	cacheExports := cfg.ExportCaches
	cacheImports := cfg.ImportCaches

	eg, ctx := errgroup.WithContext(ctx)

	solveOpt := client.SolveOpt{
		Exports: cfg.Exports,
		// LocalDirs is set later
		Frontend: cfg.Frontend,
		// FrontendAttrs is set later
		CacheExports:        cacheExports,
		CacheImports:        cacheImports,
		Session:             attachable,
		AllowedEntitlements: allowed,
	}

	solveOpt.FrontendAttrs = cfg.FrontendAttrs

	solveOpt.LocalDirs = cfg.LocalDirs

	var def *llb.Definition

	if cfg.NoCache {
		solveOpt.FrontendAttrs["no-cache"] = ""
	}

	// not using shared context to not disrupt display but let is finish reporting errors
	pw, err := progresswriter.NewPrinter(context.TODO(), os.Stderr, cfg.ProgressMode)
	if err != nil {
		return err
	}

	if traceEnc != nil {
		traceCh := make(chan *client.SolveStatus)
		pw = progresswriter.Tee(pw, traceCh)
		eg.Go(func() error {
			for s := range traceCh {
				if err := traceEnc.Encode(s); err != nil {
					return err
				}
			}
			return nil
		})
	}
	mw := progresswriter.NewMultiWriter(pw)

	var writers []progresswriter.Writer
	for _, at := range attachable {
		if s, ok := at.(interface {
			SetLogger(progresswriter.Logger)
		}); ok {
			w := mw.WithPrefix("", false)
			s.SetLogger(func(s *client.SolveStatus) {
				w.Status() <- s
			})
			writers = append(writers, w)
		}
	}

	eg.Go(func() error {
		defer func() {
			for _, w := range writers {
				close(w.Status())
			}
		}()
		resp, err := c.Solve(ctx, def, solveOpt, progresswriter.ResetTime(mw.WithPrefix("", false)).Status())
		if err != nil {
			return err
		}
		for k, v := range resp.ExporterResponse {
			logrus.Debugf("exporter response: %s=%s", k, v)
		}

		metadataFile := cfg.MetadataFile
		if metadataFile != "" && resp.ExporterResponse != nil {
			if err := writeMetadataFile(metadataFile, resp.ExporterResponse); err != nil {
				return err
			}
		}

		return nil
	})

	eg.Go(func() error {
		<-pw.Done()
		return pw.Err()
	})

	return eg.Wait()
}

func writeMetadataFile(filename string, exporterResponse map[string]string) error {
	var err error
	out := make(map[string]interface{})
	for k, v := range exporterResponse {
		dt, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			out[k] = v
			continue
		}
		var raw map[string]interface{}
		if err = json.Unmarshal(dt, &raw); err != nil || len(raw) == 0 {
			out[k] = v
			continue
		}
		out[k] = json.RawMessage(dt)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return continuity.AtomicWriteFile(filename, b, 0666)
}
