package main

import (
	"log"
	"os"

	"github.com/BehzadE/go-wikidump/pkg/wikidump"
	"github.com/BehzadE/go-wikidump/pkg/wikitext"

	"github.com/sourcegraph/zoekt"
)

func main() {
	dump, err := wikidump.New("./")
	if err != nil {
		log.Panicln(err)
	}
	err = dump.PopulateDB()
	if err != nil {
		log.Fatal(err)
	}
	reader, err := dump.NewStreamReader("simplewiki-latest-pages-articles-multistream.xml.bz2")
	if err != nil {
		log.Panicln(err)
	}
	ib, err := zoekt.NewIndexBuilder(&zoekt.Repository{})
	if err != nil {
		panic(err)
	}

	for reader.Next() {
		b, err := reader.Read()
		if err != nil {
			log.Fatal(err)
		}
		pages, err := wikidump.ParseStream(b)
		if err != nil {
			log.Fatal(err)
		}
		for _, page := range pages {
			ib.Add(zoekt.Document{
				Name:     page.Title,
				Language: "English",
				Content:  []byte(wikitext.ToPlain(page.Revision.Text)),
			})
		}
	}

	f, err := os.Create("wiki.index")
	if err != nil {
		log.Panicln(err)
	}
	if err := ib.Write(f); err != nil {
		log.Panicln(err)
	}
	_ = f.Close()

}
