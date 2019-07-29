package gobac

import (
	"log"
	"sync"
	"time"
)

var transactions = make(map[uint8]*Transaction)
var mutex sync.RWMutex

type TransactionHandler = func(data []byte)

type Transaction struct {
	InvokeID uint8
	Handler  TransactionHandler
}

func NewTransaction(handler TransactionHandler) *Transaction {
	invokeId := -1

	mutex.RLock()
	for i := 0; i < 255; i++ {
		if _, exists := transactions[uint8(i)]; exists {
			continue
		}

		invokeId = i
		break
	}
	mutex.RUnlock()

	if invokeId < 0 {
		log.Println("There isn't an invoke ID available, sleeping for 3 seconds and retrying...")
		time.Sleep(time.Second * 3)
		return NewTransaction(handler)
	}

	t := &Transaction{
		InvokeID: uint8(invokeId),
		Handler:  handler,
	}

	mutex.Lock()
	transactions[t.InvokeID] = t
	mutex.Unlock()

	return t
}

func (t *Transaction) Release() {
	mutex.Lock()
	delete(transactions, t.InvokeID)
	mutex.Unlock()
}
