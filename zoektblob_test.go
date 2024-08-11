package zoektblob

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
	"golang.org/x/exp/constraints"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

var _ zoekt.IndexFile = &wrappedIndex{}

func copySet(s map[uint32]struct{}) map[uint32]struct{} {
	r := map[uint32]struct{}{}
	for k := range s {
		r[k] = struct{}{}
	}
	return r
}

func setDifference(a, b map[uint32]struct{}) map[uint32]struct{} {
	result := map[uint32]struct{}{}
	for k := range b {
		if _, ok := a[k]; !ok {
			result[k] = struct{}{}
		}
	}
	return result
}

func uniqueTrigrams(data []byte) []string {
	trigrams := make(map[string]struct{})

	for i := 0; i < len(data)-2; i++ {
		trigram := string(data[i : i+3])
		trigrams[trigram] = struct{}{}
	}

	unique := make([]string, 0, len(trigrams))
	for trigram := range trigrams {
		unique = append(unique, trigram)
	}
	sort.Strings(unique)
	subset := make([]string, 0)
	for i := 0; i < len(trigrams); i += 100 {
		subset = append(subset, unique[i])
	}

	return subset
}

func generateTrigrams() ([]string, error) {
	f, err := os.Open("wiki.index")
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	defer f.Close()
	idxf, err := zoekt.NewIndexFile(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create index file: %w", err)
	}
	defer idxf.Close()
	searcher, err := zoekt.NewSearcher(idxf)
	if err != nil {
		return nil, fmt.Errorf("failed to create searcher: %w", err)
	}
	defer searcher.Close()
	result, err := searcher.Search(context.Background(), &query.Substring{
		Pattern:  "April",
		FileName: true,
	}, &zoekt.SearchOptions{
		Whole: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	return uniqueTrigrams(result.Files[0].Content), nil
}

type Output struct {
	chunkSize uint32
	numChunks int

	indexChunks int

	avgNewSearchChunks float64
}

func prettyMb[t constraints.Integer | constraints.Float](n t) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%.2fMB", float64(n)/1e6)
}

func (o Output) String() string {
	return fmt.Sprintf("chunkSize: %s, numChunks: %d, indexChunks: %d, indexSize: %s, avgNewSearchChunks: %.2f, searchChunkSize: %s",
		prettyMb(o.chunkSize),
		o.numChunks,
		o.indexChunks,
		prettyMb(o.chunkSize*uint32(o.indexChunks)),
		o.avgNewSearchChunks,
		prettyMb(o.avgNewSearchChunks*float64(o.chunkSize)),
	)
}

func TestIndex(t *testing.T) {

	trigrams, err := generateTrigrams()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(len(trigrams))

	f, err := os.Open("wiki.index")
	if err != nil {
		t.Fatal(err)
	}
	idxf, err := zoekt.NewIndexFile(f)
	if err != nil {
		t.Fatal(err)
	}

	out := []Output{}
	for i := 28; i < 100; i++ {
		o := Output{}
		widx := &wrappedIndex{idxf, map[uint32]struct{}{}, 0, 0.25e6 * uint32(i)}
		o.chunkSize = widx.chunkSize

		size, _ := widx.Size()
		o.numChunks = int(size / widx.chunkSize)
		searcher, err := zoekt.NewSearcher(widx)
		if err != nil {
			t.Fatal(err)
		}

		for _, trigram := range trigrams[:1] {
			// Warm common search paths (btree except leaves?)
			if _, err := searcher.Search(context.Background(), &query.Substring{
				Pattern: trigram,
			}, &zoekt.SearchOptions{
				Whole: false,
				// EstimateDocCount:   true,
				// TotalMaxMatchCount: 10,
				// MaxDocDisplayCount: 10,
			}); err != nil {
				t.Fatal(err)
			}
		}

		o.indexChunks = len(widx.pagesTouched)
		// fmt.Println(widx.pagesTouched, " - ", len(widx.pagesTouched)*int(widx.chunkSize)/1e6, "MB")
		indexChunks := copySet(widx.pagesTouched)
		widx.pagesTouched = map[uint32]struct{}{}

		blockingChunksCount := 0
		for _, trigram := range trigrams {
			t0 := time.Now()
			result, err := searcher.Search(context.Background(), &query.Substring{
				Pattern: trigram,
			}, &zoekt.SearchOptions{
				Whole: false,
				// EstimateDocCount:   true,
				// TotalMaxMatchCount: 10,
				// MaxDocDisplayCount: 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			touched := setDifference(indexChunks, widx.pagesTouched)
			fmt.Println(touched)
			_, _ = t0, result
			// fmt.Println(len(result.Files), " - ", result.FileCount, result.MatchCount)
			// fmt.Println("Search time: ", time.Since(t0))
			// fmt.Println(touched, " - ", (len(touched)*int(widx.chunkSize))/1e6, "MB")
			blockingChunksCount += len(touched)
			widx.pagesTouched = map[uint32]struct{}{}
		}
		o.avgNewSearchChunks = float64(blockingChunksCount) / float64(len(trigrams))
		fmt.Println(o)
		out = append(out, o)
	}

}
