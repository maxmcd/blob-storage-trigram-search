package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
)

type wrappedIndex struct {
	zoekt.IndexFile

	pagesTouched map[uint32]struct{}

	bytesRead uint32
	chunkSize uint32
}

func (f *wrappedIndex) Name() string          { return f.IndexFile.Name() }
func (f *wrappedIndex) Size() (uint32, error) { return f.IndexFile.Size() }
func (f *wrappedIndex) Close()                { f.IndexFile.Close() }

func (f *wrappedIndex) Read(off, sz uint32) ([]byte, error) {
	f.pagesTouched[off/f.chunkSize] = struct{}{}
	return f.IndexFile.Read(off, sz)
}

func (f *wrappedIndex) PrintPagesTouched() {
	fmt.Println(f.pagesTouched, " - ", len(f.pagesTouched)*int(f.chunkSize)/1e6, "MB")
	f.pagesTouched = map[uint32]struct{}{}

}

var _ zoekt.IndexFile = &wrappedIndex{}

func TestIndex(t *testing.T) {
	t0 := time.Now()
	f, err := os.Open("wiki.index")
	if err != nil {
		t.Fatal(err)
	}
	idxf, err := zoekt.NewIndexFile(f)
	if err != nil {
		t.Fatal(err)
	}
	widx := &wrappedIndex{idxf, map[uint32]struct{}{}, 0, 0.5e6}
	size, _ := widx.Size()
	fmt.Println("Num chunks", size/widx.chunkSize)
	searcher, err := zoekt.NewSearcher(widx)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("Index loaded in", time.Since(t0))
	t1 := time.Now()

	widx.PrintPagesTouched()

	result, err := searcher.Search(context.Background(), &query.Substring{
		Pattern:  "and",
		FileName: true,
	}, &zoekt.SearchOptions{
		// Whole: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Search completed in", time.Since(t1))
	widx.PrintPagesTouched()

	files := result.Files
	if len(files) > 15 {
		files = files[:15]
	}
	for _, file := range files {
		t.Logf("File: %s\n", file.FileName)
		// t.Logf("Content: %s\n", string(file.Content))
		result, err := searcher.Search(context.Background(), &query.FileNameSet{
			Set: map[string]struct{}{file.FileName: {}},
		}, &zoekt.SearchOptions{
			Whole: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(result.Files[0].FileName, len(result.Files[0].Content))
		widx.PrintPagesTouched()
	}
}
