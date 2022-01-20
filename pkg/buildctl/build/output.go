package build

import (
	"io"
	"os"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/pkg/errors"
)

func ResolveExporterDest(exporter, dest string) (func(map[string]string) (io.WriteCloser, error), string, error) {
	wrapWriter := func(wc io.WriteCloser) func(map[string]string) (io.WriteCloser, error) {
		return func(m map[string]string) (io.WriteCloser, error) {
			return wc, nil
		}
	}
	switch exporter {
	case client.ExporterLocal:
		if dest == "" {
			return nil, "", errors.New("output directory is required for local exporter")
		}
		return nil, dest, nil
	case client.ExporterOCI, client.ExporterDocker, client.ExporterTar:
		if dest != "" && dest != "-" {
			fi, err := os.Stat(dest)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, "", errors.Wrapf(err, "invalid destination file: %s", dest)
			}
			if err == nil && fi.IsDir() {
				return nil, "", errors.Errorf("destination file is a directory")
			}
			w, err := os.Create(dest)
			return wrapWriter(w), "", err
		}
		// if no output file is specified, use stdout
		if _, err := console.ConsoleFromFile(os.Stdout); err == nil {
			return nil, "", errors.Errorf("output file is required for %s exporter. refusing to write to console", exporter)
		}
		return wrapWriter(os.Stdout), "", nil
	default: // e.g. client.ExporterImage
		if dest != "" {
			return nil, "", errors.Errorf("output %s is not supported by %s exporter", dest, exporter)
		}
		return nil, "", nil
	}
}
