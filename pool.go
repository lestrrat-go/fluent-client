package fluent

import "sync"

var msgpool = sync.Pool{
	New: allocMessage,
}

func allocMessage() interface{} {
	return &Message{}
}

func getMessage() *Message {
	return msgpool.Get().(*Message)
}

func releaseMessage(m *Message) {
	m.Tag = ""
	m.Time = EventTime{}
	m.Record = nil
	m.Option = nil
	if m.replyCh != nil {
		close(m.replyCh)
		m.replyCh = nil
	}
	msgpool.Put(m)
}
