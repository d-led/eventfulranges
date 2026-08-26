// Package meta merges JSON-object metadata attached to ranges. Metadata is a
// JSON object; merging two objects is itself a CRDT join, so a Merge must be
// commutative, associative and idempotent for replicas to converge regardless
// of the order in which operations arrive.
package meta

import (
	"bytes"
	"encoding/json"
)

// Merge joins two metadata objects into one. Implementations must be
// commutative, associative and idempotent — the same contract as any CRDT
// join — so that the metadata carried by merged ranges converges.
type Merge func(a, b json.RawMessage) json.RawMessage

// Union merges top-level keys: a key present in only one object is kept, equal
// values collapse, and conflicting values resolve by recursing into nested
// objects or, for scalars, by keeping the lexicographically larger JSON. The
// scalar rule is deterministic and order-independent, which makes Union a
// valid CRDT join; a richer JSON CRDT can replace it behind the same Merge
// interface.
func Union(a, b json.RawMessage) json.RawMessage {
	ao := object(a)
	if ao == nil {
		return b
	}
	bo := object(b)
	if bo == nil {
		return a
	}
	return mergeObjects(ao, bo)
}

// resolve picks a winner for one key carried by both objects.
func resolve(a, b json.RawMessage) json.RawMessage {
	if bytes.Equal(a, b) {
		return a
	}
	if ao, bo := object(a), object(b); ao != nil && bo != nil {
		return mergeObjects(ao, bo)
	}
	if bytes.Compare(a, b) > 0 {
		return a
	}
	return b
}

// mergeObjects joins two parsed objects into one.
func mergeObjects(a, b map[string]json.RawMessage) json.RawMessage {
	out := make(map[string]json.RawMessage, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		prev, ok := out[k]
		if !ok {
			out[k] = v
			continue
		}
		out[k] = resolve(prev, v)
	}
	return marshal(out)
}

// object parses raw as a JSON object, or returns nil when it is empty or is
// not an object.
func object(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// marshal renders a map deterministically: encoding/json sorts map keys, so
// equal objects always marshal to byte-identical output, keeping the scalar
// tie-break in resolve stable.
func marshal(m map[string]json.RawMessage) json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}
