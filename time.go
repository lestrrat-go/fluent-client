package fluent

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/pkg/errors"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

var _ msgpack.Marshaler = (*EventTime)(nil)
var _ msgpack.Unmarshaler = (*EventTime)(nil)

func init() {
	msgpack.RegisterExt(0, &EventTime{})
}

func (tm *EventTime) MarshalMsgpack() ([]byte, error) {
	b := make([]byte, 8)

	binary.BigEndian.PutUint32(b[:4], uint32(tm.Unix()))
	nsec := tm.Nanosecond()
	for nsec > math.MaxInt32 {
		nsec /= 10
	}
	binary.BigEndian.PutUint32(b[4:], uint32(nsec))
	return b, nil
}

func (tm *EventTime) UnmarshalMsgpack(b []byte) error {
	if len(b) != 8 {
		return errors.Errorf("invalid data length: got %d, wanted 8", len(b))
	}
	sec := binary.BigEndian.Uint32(b)
	usec := binary.BigEndian.Uint32(b[4:])
	tm.Time = time.Unix(int64(sec), int64(usec))
	return nil
}
