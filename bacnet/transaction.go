package bacnet

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
)

var ErrInvokeIDExhausted = errors.New("no BACnet invoke ID is available")

var transactions = make(map[string]bool)
var transactionNext = make(map[string]uint8)
var mtx sync.RWMutex

func genTxId(addressStr string, num uint8) string {
	return strings.Join([]string{
		addressStr, strconv.Itoa(int(num)),
	}, ".")
}

func GetInvokeID(address net.IP) uint8 {
	invokeID, _ := TryGetInvokeID(address)
	return invokeID
}

// TryGetInvokeID reserves an invoke ID for a destination. Invoke ID zero is
// reserved as the invalid/no-ID value, matching the BACnet transaction model.
func TryGetInvokeID(address net.IP) (uint8, error) {
	if address == nil {
		return 0, errors.New("received a nil device IP")
	}

	var addressStr = address.String()

	mtx.Lock()
	defer mtx.Unlock()

	invokeID := transactionNext[addressStr]
	if invokeID == 0 {
		invokeID = 1
	}
	for i := 0; i < 255; i++ {
		strTx := genTxId(addressStr, invokeID)

		if !transactions[strTx] {
			transactions[strTx] = true
			next := invokeID + 1
			if next == 0 {
				next = 1
			}
			transactionNext[addressStr] = next
			return invokeID, nil
		}
		invokeID++
		if invokeID == 0 {
			invokeID = 1
		}
	}

	return 0, ErrInvokeIDExhausted
}

func ReleaseInvokeID(address net.IP, invokeId uint8) {
	if address == nil || invokeId == 0 {
		return
	}

	strInvokeId := genTxId(address.String(), invokeId)

	mtx.Lock()
	defer mtx.Unlock()

	delete(transactions, strInvokeId)
}
