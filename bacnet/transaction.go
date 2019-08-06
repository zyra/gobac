package bacnet

import (
	"sync"
)

var transactions = make([]bool, 255)
var mtx sync.RWMutex

func GetInvokeID() uint8 {
	var invokeId uint8 = 0
	var i uint8

	mtx.Lock()
	for i = 1; i < 255; i++ {
		if exists := transactions[uint8(i)]; exists {
			continue
		}
		invokeId = i
		transactions[invokeId] = true
		break
	}
	mtx.Unlock()

	if invokeId == 0 {
		//log.Println("There isn't an invoke ID available, sleeping for 3 seconds and retrying...")
		return GetInvokeID()
	}

	return invokeId
}

func ReleaseInvokeID(invokeId uint8) {
	if invokeId == 0 {
		return
	}

	mtx.Lock()
	transactions[invokeId] = false
	mtx.Unlock()
}
