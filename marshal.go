package fluent

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

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
		Time:   t,
		Record: r,
		Option: o,
	}

	return nil
}

func (m *Message) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte('[')
	buf.WriteString(strconv.Quote(m.Tag))
	buf.WriteByte(',')
	buf.WriteString(strconv.FormatInt(m.Time, 10))
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

	return buf.Bytes(), nil
}

func msgpackMarshal(tag string, t int64, record, option interface{}) ([]byte, error) {
	return msgpack.Marshal(&Message{
		Tag:    tag,
		Time:   t,
		Record: record,
		Option: option,
	})
}

func jsonMarshal(tag string, t int64, record, option interface{}) ([]byte, error) {
	return json.Marshal(&Message{
		Tag:    tag,
		Time:   t,
		Record: record,
		Option: option,
	})
}
