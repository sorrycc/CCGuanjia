// Package jsonedit edits the keys of a JSON object and leaves the rest of
// the file as it was: the order of the keys, and every value it does not
// touch, byte for byte. Claude Code's .claude.json and settings.json are
// edited with it, so that a change to one key never reorders the others.
package jsonedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Object is a JSON object as its keys come, in order.
type Object struct {
	keys   []string
	values map[string]json.RawMessage
}

// Parse reads a JSON object.
func Parse(data []byte) (*Object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("not a JSON object")
	}
	o := &Object{values: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("a key that is not a string: %v", tok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := o.values[key]; !dup {
			o.keys = append(o.keys, key)
		}
		o.values[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return o, nil
}

// Keys returns the keys in order.
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

// Has reports whether the object has key.
func (o *Object) Has(key string) bool { _, ok := o.values[key]; return ok }

// Raw returns the value of key as it is written, nil when there is none.
func (o *Object) Raw(key string) json.RawMessage { return o.values[key] }

// Get decodes the value of key into v, and reports whether there was one.
func (o *Object) Get(key string, v any) (bool, error) {
	raw, ok := o.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, v)
}

// Set sets key to v, in its place, or after the other keys when it is new.
func (o *Object) Set(key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return o.SetRaw(key, raw)
}

// SetRaw sets key to raw, which must be JSON.
func (o *Object) SetRaw(key string, raw json.RawMessage) error {
	if !json.Valid(raw) {
		return fmt.Errorf("the value of %q is not JSON", key)
	}
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = append(json.RawMessage(nil), raw...)
	return nil
}

// Delete removes key, and reports whether it was there.
func (o *Object) Delete(key string) bool {
	if _, ok := o.values[key]; !ok {
		return false
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
	return true
}

// Marshal writes the object indented by indent, as json.MarshalIndent
// does, without escaping <, > and &, as Claude Code writes its files.
func (o *Object) Marshal(indent string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		var kb bytes.Buffer
		enc := json.NewEncoder(&kb)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(k); err != nil {
			return nil, err
		}
		b.Write(bytes.TrimRight(kb.Bytes(), "\n"))
		b.WriteByte(':')
		b.Write(o.values[k])
	}
	b.WriteByte('}')
	var out bytes.Buffer
	if err := json.Indent(&out, b.Bytes(), "", indent); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Indent returns the indentation of the first indented line of data, two
// spaces when there is none: the indent to write a file back with.
func Indent(data []byte) string {
	for _, line := range strings.Split(string(data), "\n")[1:] {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && len(trimmed) < len(line) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "  "
}
