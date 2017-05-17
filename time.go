package fluent

import (
	"time"

	msgpack "github.com/lestrrat/go-msgpack"
	"github.com/pkg/errors"
)

func init() {
	if err := msgpack.RegisterExt(0, EventTime{}); err != nil {
		panic(err)
	}
}

func (t *EventTime) DecodeMsgpack(d *msgpack.Decoder) error {
	code, err := d.ReadCode()
	if err != nil {
		return errors.Wrap(err, `failed to read code`)
	}

	r := d.Reader()
	switch code {
	case msgpack.FixExt8:
		// no op. we know we have 8 bytes
	case msgpack.Ext8:
		size, err := r.ReadUint8()
		if err != nil {
			return errors.Wrap(err, `failed to read ext8 length`)
		}

		if size != 8 {
			return errors.Wrap(err, `expected 8 bytes payload`)
		}
	default:
		return errors.Errorf(`invalid code %s`, code)
	}

	typ, err := r.ReadUint8()
	if typ != 0 {
		return errors.Errorf(`invalid ext type %d`, typ)
	}

	sec, err := r.ReadUint32()
	if err != nil {
		return errors.Wrap(err, `failed to read uint32 from first 4 bytes`)
	}

	nsec, err := r.ReadUint32()
	if err != nil {
		return errors.Wrap(err, `failed to read uint32 from second 4 bytes`)
	}

	t.Time = time.Unix(int64(sec), int64(nsec)).UTC()
	return nil
}

func (t EventTime) EncodeMsgpack(e *msgpack.Encoder) error {
	e.EncodeExtHeader(8)
	e.EncodeExtType(t)

	w := e.Writer()
	if err := w.WriteUint32(uint32(t.Unix())); err != nil {
		return errors.Wrap(err, `failed to write EventTime seconds payload`)
	}

	if err := w.WriteUint32(uint32(t.Nanosecond())); err != nil {
		return errors.Wrap(err, `failed to write EventTime nanoseconds payload`)
	}

	return nil
}
