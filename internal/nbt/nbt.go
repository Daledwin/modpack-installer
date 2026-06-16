// Package nbt is a minimal NBT codec, enough to read and rewrite servers.dat
// (uncompressed NBT) while preserving any tags/servers we don't manage.
package nbt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// Tag type IDs.
const (
	TagEnd       = 0
	TagByte      = 1
	TagShort     = 2
	TagInt       = 3
	TagLong      = 4
	TagFloat     = 5
	TagDouble    = 6
	TagByteArray = 7
	TagString    = 8
	TagList      = 9
	TagCompound  = 10
	TagIntArray  = 11
	TagLongArray = 12
)

// Compound is an ordered set of named tags (order preserved for round-trips).
type Compound struct {
	Names []string
	Vals  []any
}

func (c *Compound) Get(name string) (any, bool) {
	for i, n := range c.Names {
		if n == name {
			return c.Vals[i], true
		}
	}
	return nil, false
}

// Set replaces an existing key or appends a new one.
func (c *Compound) Set(name string, v any) {
	for i, n := range c.Names {
		if n == name {
			c.Vals[i] = v
			return
		}
	}
	c.Names = append(c.Names, name)
	c.Vals = append(c.Vals, v)
}

// List is a typed sequence of payloads.
type List struct {
	ElemType byte
	Items    []any
}

// ---- decode ----------------------------------------------------------------

type decoder struct {
	b []byte
	i int
}

func (d *decoder) need(n int) error {
	if d.i+n > len(d.b) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (d *decoder) u8() (byte, error) {
	if err := d.need(1); err != nil {
		return 0, err
	}
	v := d.b[d.i]
	d.i++
	return v, nil
}
func (d *decoder) read(n int) ([]byte, error) {
	if err := d.need(n); err != nil {
		return nil, err
	}
	v := d.b[d.i : d.i+n]
	d.i += n
	return v, nil
}
func (d *decoder) str() (string, error) {
	h, err := d.read(2)
	if err != nil {
		return "", err
	}
	n := int(binary.BigEndian.Uint16(h))
	s, err := d.read(n)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func (d *decoder) payload(id byte) (any, error) {
	switch id {
	case TagByte:
		v, err := d.u8()
		return int8(v), err
	case TagShort:
		b, err := d.read(2)
		if err != nil {
			return nil, err
		}
		return int16(binary.BigEndian.Uint16(b)), nil
	case TagInt:
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		return int32(binary.BigEndian.Uint32(b)), nil
	case TagLong:
		b, err := d.read(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(b)), nil
	case TagFloat:
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		return math.Float32frombits(binary.BigEndian.Uint32(b)), nil
	case TagDouble:
		b, err := d.read(8)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	case TagString:
		return d.str()
	case TagByteArray:
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		n := int(int32(binary.BigEndian.Uint32(b)))
		raw, err := d.read(n)
		if err != nil {
			return nil, err
		}
		out := make([]byte, n)
		copy(out, raw)
		return out, nil
	case TagIntArray:
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		n := int(int32(binary.BigEndian.Uint32(b)))
		arr := make([]int32, n)
		for k := 0; k < n; k++ {
			e, err := d.read(4)
			if err != nil {
				return nil, err
			}
			arr[k] = int32(binary.BigEndian.Uint32(e))
		}
		return arr, nil
	case TagLongArray:
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		n := int(int32(binary.BigEndian.Uint32(b)))
		arr := make([]int64, n)
		for k := 0; k < n; k++ {
			e, err := d.read(8)
			if err != nil {
				return nil, err
			}
			arr[k] = int64(binary.BigEndian.Uint64(e))
		}
		return arr, nil
	case TagList:
		et, err := d.u8()
		if err != nil {
			return nil, err
		}
		b, err := d.read(4)
		if err != nil {
			return nil, err
		}
		n := int(int32(binary.BigEndian.Uint32(b)))
		lst := &List{ElemType: et, Items: make([]any, 0, n)}
		for k := 0; k < n; k++ {
			v, err := d.payload(et)
			if err != nil {
				return nil, err
			}
			lst.Items = append(lst.Items, v)
		}
		return lst, nil
	case TagCompound:
		c := &Compound{}
		for {
			t, err := d.u8()
			if err != nil {
				return nil, err
			}
			if t == TagEnd {
				break
			}
			name, err := d.str()
			if err != nil {
				return nil, err
			}
			v, err := d.payload(t)
			if err != nil {
				return nil, err
			}
			c.Names = append(c.Names, name)
			c.Vals = append(c.Vals, v)
		}
		return c, nil
	default:
		return nil, fmt.Errorf("nbt: unknown tag id %d", id)
	}
}

// Decode parses a root NBT document and returns the root compound.
func Decode(data []byte) (*Compound, error) {
	// servers.dat is uncompressed, but tolerate gzip just in case.
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		dec, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}
		data = dec
	}
	d := &decoder{b: data}
	id, err := d.u8()
	if err != nil {
		return nil, err
	}
	if id != TagCompound {
		return nil, fmt.Errorf("nbt: root is not a compound (id %d)", id)
	}
	if _, err := d.str(); err != nil { // root name (usually "")
		return nil, err
	}
	v, err := d.payload(TagCompound)
	if err != nil {
		return nil, err
	}
	return v.(*Compound), nil
}

// ---- encode ----------------------------------------------------------------

func tagID(v any) (byte, error) {
	switch v.(type) {
	case int8:
		return TagByte, nil
	case int16:
		return TagShort, nil
	case int32:
		return TagInt, nil
	case int64:
		return TagLong, nil
	case float32:
		return TagFloat, nil
	case float64:
		return TagDouble, nil
	case []byte:
		return TagByteArray, nil
	case string:
		return TagString, nil
	case *List:
		return TagList, nil
	case *Compound:
		return TagCompound, nil
	case []int32:
		return TagIntArray, nil
	case []int64:
		return TagLongArray, nil
	default:
		return 0, fmt.Errorf("nbt: cannot encode %T", v)
	}
}

func writeStr(buf *bytes.Buffer, s string) {
	var h [2]byte
	binary.BigEndian.PutUint16(h[:], uint16(len(s)))
	buf.Write(h[:])
	buf.WriteString(s)
}

func writePayload(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case int8:
		buf.WriteByte(byte(t))
	case int16:
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(t))
		buf.Write(b[:])
	case int32:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(t))
		buf.Write(b[:])
	case int64:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(t))
		buf.Write(b[:])
	case float32:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], math.Float32bits(t))
		buf.Write(b[:])
	case float64:
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], math.Float64bits(t))
		buf.Write(b[:])
	case []byte:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(t)))
		buf.Write(b[:])
		buf.Write(t)
	case string:
		writeStr(buf, t)
	case []int32:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(t)))
		buf.Write(b[:])
		for _, e := range t {
			var eb [4]byte
			binary.BigEndian.PutUint32(eb[:], uint32(e))
			buf.Write(eb[:])
		}
	case []int64:
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(t)))
		buf.Write(b[:])
		for _, e := range t {
			var eb [8]byte
			binary.BigEndian.PutUint64(eb[:], uint64(e))
			buf.Write(eb[:])
		}
	case *List:
		et := t.ElemType
		if et == TagEnd && len(t.Items) > 0 {
			id, err := tagID(t.Items[0])
			if err != nil {
				return err
			}
			et = id
		}
		buf.WriteByte(et)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(t.Items)))
		buf.Write(b[:])
		for _, it := range t.Items {
			if err := writePayload(buf, it); err != nil {
				return err
			}
		}
	case *Compound:
		for i, name := range t.Names {
			id, err := tagID(t.Vals[i])
			if err != nil {
				return err
			}
			buf.WriteByte(id)
			writeStr(buf, name)
			if err := writePayload(buf, t.Vals[i]); err != nil {
				return err
			}
		}
		buf.WriteByte(TagEnd)
	default:
		return fmt.Errorf("nbt: cannot encode %T", v)
	}
	return nil
}

// Encode serializes a root compound to uncompressed NBT bytes.
func Encode(root *Compound) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(TagCompound)
	writeStr(&buf, "") // root name
	if err := writePayload(&buf, root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadFile decodes an NBT file, or returns an empty root compound if absent.
func ReadFile(path string) (*Compound, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Compound{}, nil
		}
		return nil, err
	}
	return Decode(data)
}

// WriteFile encodes the root compound to an uncompressed NBT file.
func WriteFile(path string, root *Compound) error {
	out, err := Encode(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
