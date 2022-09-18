package internal

import "io"

type OpenedFile struct {
	// Open readers late so it is less likely that we hit OS limits on opened files
	LateReader func() (io.ReadCloser, error)
	Filename   string
}
