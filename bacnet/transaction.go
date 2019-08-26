package bacnet

import (
	"net"
	"strconv"
	"strings"
	"sync"
)

var transactions = make(map[string]bool)
var mtx sync.RWMutex

func genTxId(addressStr string, num uint8) string {
	return strings.Join([]string{
		addressStr, strconv.Itoa(int(num)),
	}, ".")
}

func GetInvokeID(address *net.IP) uint8 {
	var addressStr = address.String()
	var invokeId uint8 = 0
	var i uint8
	var strTx string

	mtx.Lock()
	defer mtx.Unlock()

	for i = 1; i < 255; i++ {
		strTx = genTxId(addressStr, i)

		if val, ok := transactions[strTx]; ok && val {
			continue
		}

		invokeId = i
		transactions[strTx] = true
		return invokeId
	}

	return GetInvokeID(address)
}

func ReleaseInvokeID(address *net.IP, invokeId uint8) {
	if address == nil || invokeId == 0 {
		return
	}

	strInvokeId := genTxId(address.String(), invokeId)

	mtx.Lock()
	defer mtx.Unlock()

	if _, ok := transactions[strInvokeId]; ok {
		transactions[strInvokeId] = false
	}
}
