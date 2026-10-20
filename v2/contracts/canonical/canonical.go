// Package canonical encodes JSON the way RFC 8785 (JCS) does for the values
// the contract uses: object keys sorted, no insignificant whitespace, no
// HTML escaping, and integers written as they are. The manifest signature
// and the event hash chain are computed over this form, on both sides.
package canonical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Marshal returns v's canonical JSON. v is first encoded with encoding/json,
// so struct tags apply.
func Marshal(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return FromJSON(raw)
}

// FromJSON re-encodes JSON text canonically.
func FromJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := write(&b, x); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// MarshalWithout is Marshal with top-level keys removed (e.g. "signature").
func MarshalWithout(v any, drop ...string) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	for _, k := range drop {
		delete(m, k)
	}
	var b bytes.Buffer
	if err := write(&b, m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func write(b *bytes.Buffer, x any) error {
	switch v := x.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(v.String())
	case string:
		writeString(b, v)
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := write(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			if err := write(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("canonical: unexpected %T", x)
	}
	return nil
}

func writeString(b *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
}
