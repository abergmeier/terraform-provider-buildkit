package digest

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"crypto/sha512"

	"github.com/abergmeier/buildkit_ex/internal"
	internalio "github.com/abergmeier/buildkit_ex/internal/io"
	"github.com/abergmeier/buildkit_ex/pkg/digest/options"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

type sourceContentWithIndex struct {
	i  int
	sc instructions.SourceContent
}

func DigestOfFileAndAllInputs(filename string, opts ...options.Option) ([sha512.Size]byte, error) {

	digest := [sha512.Size]byte{}

	dt, err := os.ReadFile(filename)
	if err != nil {
		return digest, fmt.Errorf("Reading Containerfile failed: %w", err)
	}

	filesToHash := make(chan internalio.OpenedFile, 1024)
	var herr error
	digestReady := sync.WaitGroup{}
	digestReady.Add(1)
	go func() {
		defer digestReady.Done()
		var h []byte
		h, herr = hashFiles(filesToHash)
		if herr != nil {
			return
		}
		copy(digest[:], h)
	}()

	filesToHash <- internalio.OpenedFile{
		LateReader: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(string(dt))), nil
		},
		Filename: filename,
	}

	rf := &internal.ReferencedFiles{}

	for _, opt := range opts {
		opt.InitReferencedFiles(rf)
	}

	err = rf.CollectSortedFileHashes(dt, filesToHash)
	if err != nil {
		return digest, err
	}

	close(filesToHash)

	digestReady.Wait()
	return digest, herr
}

func hashFiles(files <-chan internalio.OpenedFile) (sum []byte, firstErr error) {
	hash := sha512.New()

	for f := range files {
		reader, err := f.LateReader()
		if err != nil {
			break
		}
		_, err = io.Copy(hash, reader)
		reader.Close()
		if err != nil {
			break
		}
	}

	sum = hash.Sum(nil)
	return
}
