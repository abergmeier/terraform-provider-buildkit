package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	internalio "github.com/abergmeier/buildkit_ex/internal/io"
	"github.com/abergmeier/buildkit_ex/internal/ioslices"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

type ReferencedFiles struct {
	DownloadHttp    bool
	HttpAsUnchanged bool
}

func (r *ReferencedFiles) CollectSortedFileHashes(dt []byte, filesToHash chan<- internalio.OpenedFile) error {

	dockerfile, err := parser.Parse(bytes.NewReader(dt))
	if err != nil {
		return fmt.Errorf("Parsing Dockerfile failed: %w", err)
	}

	stages, _, err := instructions.Parse(dockerfile.AST)
	if err != nil {
		return fmt.Errorf("Parsing Dockerfile AST failed: %w", err)
	}

	sourcePaths := make(chan string, 1024)
	unorderedFileHashes := make(chan internalio.OpenedFile, 1024)

	go func() {
		defer close(unorderedFileHashes)
		r.hashFileContent(sourcePaths, 4, unorderedFileHashes)
	}()

	go func() {
		defer close(sourcePaths)
		collectSourcePaths(stages, sourcePaths)
	}()

	sorted := ioslices.OpenedFileSlice{}

	for fh := range unorderedFileHashes {
		sorted = append(sorted, fh)
	}
	sort.Sort(sorted)

	for _, s := range sorted {
		filesToHash <- s
	}

	return nil
}

func (r *ReferencedFiles) hashFileContent(sourcePaths <-chan string, concurrency int, referencedFiles chan<- internalio.OpenedFile) error {

	if r.DownloadHttp {
		panic("Downloading http not implemented yet")
	}

	wg := sync.WaitGroup{}
	wg.Add(concurrency)

	var cerr error

	for i := 0; i != concurrency; i++ {
		go func() {
			defer wg.Done()
			for sp := range sourcePaths {
				var lateReader func() (io.ReadCloser, error)
				var err error
				if strings.HasPrefix(sp, "http://") || strings.HasPrefix(sp, "http://") {
					lateReader = r.httpUrlLateReader(sp)
					referencedFiles <- internalio.OpenedFile{
						LateReader: lateReader,
						Filename:   sp,
					}
					return
				}

				err = recursiveLocalLateReaders(sp, referencedFiles)
				if err != nil {
					cerr = err
					return
				}
			}
		}()
	}
	wg.Wait()
	return cerr
}

func (r *ReferencedFiles) httpUrlLateReader(fileUrl string) func() (io.ReadCloser, error) {
	var content string
	if r.HttpAsUnchanged {
		content = fileUrl
	} else {
		content = fmt.Sprintf("%s", time.Now())
	}

	return func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}
}

func recursiveLocalLateReaders(path string, referencedFiles chan<- internalio.OpenedFile) error {
	return filepath.WalkDir(path,
		func(path string, info os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				referencedFiles <- internalio.OpenedFile{
					LateReader: func() (io.ReadCloser, error) {
						return os.Open(path)
					},
					Filename: path,
				}
			}
			return nil
		},
	)
}
