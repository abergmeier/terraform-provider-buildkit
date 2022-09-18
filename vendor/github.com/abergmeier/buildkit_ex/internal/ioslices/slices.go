package ioslices

import (
	internalio "github.com/abergmeier/buildkit_ex/internal/io"
)

type OpenedFileSlice []internalio.OpenedFile

func (s OpenedFileSlice) Len() int           { return len(s) }
func (s OpenedFileSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s OpenedFileSlice) Less(i, j int) bool { return s[i].Filename < s[j].Filename }
