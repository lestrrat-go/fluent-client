package fluent

import (
	"bytes"
	"encoding/json"

	msgpack "github.com/lestrrat/go-msgpack"
	"github.com/pkg/errors"
)

type marshalFunc func(*Message) ([]byte, error)

func (f marshalFunc) Marshal(msg *Message) ([]byte, error) {
	return f(msg)
}

func msgpackMarshal(m *Message) ([]byte, error) {
	return msgpack.Marshal(m)
}

func jsonMarshal(m *Message) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(m); err != nil {
		return nil, errors.Wrap(err, `failed to encode json`)
	}
	buf.Truncate(buf.Len() - 1) // remove new line
	return buf.Bytes(), nil
}
