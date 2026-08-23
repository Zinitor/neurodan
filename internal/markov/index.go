package markov

import (
	"encoding/gob"
	"io"
)

// Encode serializes the index to w. Building a full corpus is expensive
// (~seconds), so the builder persists it once and the generator loads it at
// startup instead of retraining.
//
// The wire format is gob-encoded map[int]map[string]map[string]int; the
// nested map types are stable across builds.
func (idx *Index) Encode(w io.Writer) error {
	return gob.NewEncoder(w).Encode(idx.m)
}

// DecodeIndex deserializes an Index previously written by Index.Encode.
func DecodeIndex(r io.Reader) (*Index, error) {
	idx := &Index{m: make(map[int]map[string]map[string]int)}
	if err := gob.NewDecoder(r).Decode(&idx.m); err != nil {
		return nil, err
	}
	if idx.m == nil {
		idx.m = make(map[int]map[string]map[string]int)
	}
	return idx, nil
}
