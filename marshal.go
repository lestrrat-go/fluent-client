package fluent

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
  msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

func (m *JSONMarshaler) Marshal(tag string, t int64, v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	buf.WriteString(strconv.Quote(tag))
	buf.WriteByte(',')
	buf.WriteString(strconv.FormatInt(t, 10))
	buf.WriteByte(',')

	// XXX Encoder appends a silly newline at the end, so use
	// json.Marshal instead
	data, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, `failed to encode record`)
	}
	buf.Write(data)
	buf.WriteString(",null]")
	return buf.Bytes(), nil
}

type msgpacked struct {
  Tag    string      `msgpack:"tag"`
  Time   int64       `msgpack:"time"`
  Record interface{} `msgpack:"record"`
  Option interface{} `msgpack:"option"`
}

func (m *MsgpackMarshaler) Marshal(tag string, t int64, v interface{}) ([]byte, error) {
	return msgpack.Marshal(msgpacked{Tag: tag, Time: t, Record: v})
}
