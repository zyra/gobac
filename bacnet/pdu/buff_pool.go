package pdu

import (
	"bytes"
	"sync"
)

var buffPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer([]byte{})
	},
}
