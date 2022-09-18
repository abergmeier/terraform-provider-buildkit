package options

import "github.com/abergmeier/buildkit_ex/internal"

type Option interface {
	InitReferencedFiles(r *internal.ReferencedFiles)
}

func TreatHttpAlwaysUnchanged() Option {
	return &treatHttpAlwaysUnchanged{}
}

func TreatHttpAlwaysChanged() Option {
	return &treatHttpAlwaysChanged{}
}

type treatHttpAlwaysUnchanged struct {
}
type treatHttpAlwaysChanged struct{}

func (o *treatHttpAlwaysUnchanged) InitReferencedFiles(r *internal.ReferencedFiles) {
	r.DownloadHttp = false
	r.HttpAsUnchanged = true
}

func (o *treatHttpAlwaysChanged) InitReferencedFiles(r *internal.ReferencedFiles) {
	r.DownloadHttp = false
	r.HttpAsUnchanged = false
}
