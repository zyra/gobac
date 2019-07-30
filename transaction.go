package gobac

import (
	"log"
	"time"
)

var transactions = make([]bool, 255)

func NewTransaction() uint8 {
	var invokeId uint8 = 0
	var i uint8

	for i = 1; i < 255; i++ {
		if exists := transactions[uint8(i)]; exists {
			continue
		}
		invokeId = i
		break
	}

	if invokeId == 0 {
		log.Println("There isn't an invoke ID available, sleeping for 3 seconds and retrying...")
		time.Sleep(time.Second * 3)
		return NewTransaction()
	}

	transactions[invokeId] = true
	return invokeId
}

func ReleaseTransaction(invokeId uint8) {
	transactions[invokeId] = false
}
