package fluent

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	msgpack "github.com/lestrrat/go-msgpack"
	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/pkg/errors"
)

type marshalFunc func(*Message) ([]byte, error)

func (f marshalFunc) Marshal(msg *Message) ([]byte, error) {
	return f(msg)
}

// UnmarshalJSON deserializes from a JSON buffer and populates
// a Message struct appropriately
func (m *Message) UnmarshalJSON(buf []byte) error {
	var l []json.RawMessage
	if err := json.Unmarshal(buf, &l); err != nil {
		return errors.Wrap(err, `failed to unmarshal JSON: expected array`)
	}

	var tag string
	if err := json.Unmarshal(l[0], &tag); err != nil {
		return errors.Wrap(err, `failed to unmarshal JSON: expected tag`)
	}

	var t int64
	if err := json.Unmarshal(l[1], &t); err != nil {
		return errors.Wrap(err, `failed to unmarshal JSON: expected timestamp`)
	}

	var r interface{}
	if err := json.Unmarshal(l[2], &r); err != nil {
		return errors.Wrap(err, `failed to unmarshal JSON: expected record`)
	}

	var o interface{}
	if err := json.Unmarshal(l[3], &o); err != nil {
		return errors.Wrap(err, `failed to unmarshal JSON: expected options`)
	}

	*m = Message{
		Tag:    tag,
		Time:   EventTime{Time: time.Unix(t, 0).UTC()},
		Record: r,
		Option: o,
	}

	return nil
}

// MarshalJSON serializes a Message to JSON format
func (m *Message) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte('[')
	buf.WriteString(strconv.Quote(m.Tag))
	buf.WriteByte(',')
	buf.WriteString(strconv.FormatInt(m.Time.Unix(), 10))
	buf.WriteByte(',')

	// XXX Encoder appends a silly newline at the end, so use
	// json.Marshal instead
	data, err := json.Marshal(m.Record)
	if err != nil {
		return nil, errors.Wrap(err, `failed to encode record`)
	}
	buf.Write(data)
	buf.WriteByte(',')

	data, err = json.Marshal(m.Option)
	if err != nil {
		return nil, errors.Wrap(err, `failed to encode option`)
	}
	buf.Write(data)
	buf.WriteByte(']')

	if pdebug.Enabled {
		pdebug.Printf("message marshaled to: %s", strconv.Quote(buf.String()))
	}

	return buf.Bytes(), nil
}

// EncodeMsgpack serializes a Message to msgpack format
func (m *Message) EncodeMsgpack(e *msgpack.Encoder) error {
	if err := e.EncodeArrayHeader(4); err != nil {
		return errors.Wrap(err, `failed to encode array header`)
	}
	if err := e.EncodeString(m.Tag); err != nil {
		return errors.Wrap(err, `failed to encode tag`)
	}

	if m.subsecond {
		if err := e.EncodeStruct(m.Time); err != nil {
			return errors.Wrap(err, `failed to encode time`)
		}
	} else {
		if err := e.EncodeInt64(m.Time.Unix()); err != nil {
			return errors.Wrap(err, `failed to encode msgpack: time`)
		}
	}

	if err := e.Encode(m.Record); err != nil {
		return errors.Wrap(err, `failed to encode record`)
	}
	if err := e.Encode(m.Option); err != nil {
		return errors.Wrap(err, `failed to encode option`)
	}
	return nil
}

// DecodeMsgpack deserializes from a msgpack buffer and populates
// a Message struct appropriately
func (m *Message) DecodeMsgpack(d *msgpack.Decoder) error {
	var l int
	if err := d.DecodeArrayLength(&l); err != nil {
		return errors.Wrap(err, `failed to decode msgpack array length`)
	}

	if l != 4 {
		return errors.Errorf(`invalid array length %d (expected 4)`, l)
	}

	if err := d.DecodeString(&m.Tag); err != nil {
		return errors.Wrap(err, `failed to decode fluentd message tag`)
	}

	c, err := d.PeekCode()
	if err != nil {
		return errors.Wrap(err, `failed to peek code for fluentd time`)
	}

	if msgpack.IsExtFamily(c) {
		if err := d.DecodeStruct(&m.Time); err != nil {
			return errors.Wrap(err, `failed to decode fluentd time`)
		}
	} else {
		switch c {
		case msgpack.Uint8:
			var v uint8
			if err := d.DecodeUint8(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Uint16:
			var v uint16
			if err := d.DecodeUint16(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Uint32:
			var v uint32
			if err := d.DecodeUint32(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Uint64:
			var v uint64
			if err := d.DecodeUint64(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Int8:
			var v int8
			if err := d.DecodeInt8(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Int16:
			var v int16
			if err := d.DecodeInt16(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Int32:
			var v int32
			if err := d.DecodeInt32(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(int64(v), 0).UTC()
		case msgpack.Int64:
			var v int64
			if err := d.DecodeInt64(&v); err != nil {
				return errors.Wrap(err, `failed to decode fluentd time`)
			}
			m.Time.Time = time.Unix(v, 0).UTC()
		}
	}

	if err := d.Decode(&m.Record); err != nil {
		return errors.Wrap(err, `failed to decode fluentd record`)
	}

	if err := d.Decode(&m.Option); err != nil {
		return errors.Wrap(err, `failed to decode fluentd option`)
	}

	return nil
}

func msgpackMarshal(m *Message) ([]byte, error) {
	var buf bytes.Buffer
	if err := msgpack.NewEncoder(&buf).Encode(m); err != nil {
		return nil, errors.Wrap(err, `failed to encode msgpack`)
	}
	return buf.Bytes(), nil
}

func jsonMarshal(m *Message) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(m); err != nil {
		return nil, errors.Wrap(err, `failed to encode json`)
	}
	buf.Truncate(buf.Len() - 1) // remove new line
	return buf.Bytes(), nil
}
