package gazerlink

import (
	"fmt"
	"sync"
)

var mtx sync.Mutex
var clients map[string]*Client

func init() {
	clients = make(map[string]*Client)
}

func GetClientClient(aesKeyHex, address string, port int) *Client {
	mtx.Lock()
	defer mtx.Unlock()

	key := address + ":" + fmt.Sprint(port)
	c, exists := clients[key]
	if exists {
		return c
	}
	c = NewClient(aesKeyHex, address, port)
	clients[key] = c
	return c
}
