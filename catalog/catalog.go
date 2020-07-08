// Code generated by running "go generate" in golang.org/x/text. DO NOT EDIT.

package catalog

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

type dictionary struct {
	index []uint32
	data  string
}

func (d *dictionary) Lookup(key string) (data string, ok bool) {
	p, ok := messageKeyToIndex[key]
	if !ok {
		return "", false
	}
	start, end := d.index[p], d.index[p+1]
	if start == end {
		return "", false
	}
	return d.data[start:end], true
}

func init() {
	dict := map[string]catalog.Dictionary{
		"en": &dictionary{index: enIndex, data: enData},
	}
	fallback := language.MustParse("en-US")
	cat, err := catalog.NewFromMap(dict, catalog.Fallback(fallback))
	if err != nil {
		panic(err)
	}
	message.DefaultCatalog = cat
}

var messageKeyToIndex = map[string]int{
	"%s is not implemented": 1,
	"not implemented":       0,
}

var enIndex = []uint32{ // 3 elements
	0x00000000, 0x00000024, 0x0000003d,
} // Size: 36 bytes

const enData string = "" + // Size: 61 bytes
	"\x02This method is not implemented yet.\x02%[1]s is not implemented"

	// Total table size 97 bytes (0KiB); checksum: 122E237E
