package zoektblob

import (
	"github.com/sourcegraph/zoekt"
)

type Index struct{}

func NewIndex() *Index {
	return &Index{}
}

var _ zoekt.IndexFile = &Index{}

func (i *Index) Name() string                        { return "" }
func (i *Index) Size() (uint32, error)               { return 0, nil }
func (i *Index) Close()                              {}
func (i *Index) Read(off, sz uint32) ([]byte, error) { return nil, nil }
