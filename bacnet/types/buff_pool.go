package types

import (
	"bytes"
	"sync"
)

var buffPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer([]byte{})
	},
}

func GetBuff(data ...byte) (buff *bytes.Buffer) {
	buff = buffPool.Get().(*bytes.Buffer)
	if len(data) > 0 {
		buff.Write(data)
	}
	return
}

func ReleaseBuff(buff *bytes.Buffer) {
	buff.Reset()
	buffPool.Put(buff)
}
