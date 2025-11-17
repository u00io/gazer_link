package gazerlink

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Server struct {
	mtx          sync.Mutex
	nextClientId uint64
	onCall       func(*Form) *Form

	aesKeyHex string
	port      int
	listener  net.Listener
	clients   map[uint64]*ConnectedClient
}

func NewServer(aesKeyHex string, port int, onCall func(*Form) *Form) *Server {
	var c Server
	c.onCall = onCall
	c.aesKeyHex = aesKeyHex
	c.port = port
	c.clients = make(map[uint64]*ConnectedClient)
	return &c
}

func (s *Server) Start() {
	go s.thWork()
}

func (s *Server) Stop() {
}

func (c *Server) thWork() {
	var err error
	for {
		c.listener, err = net.Listen("tcp", ":"+fmt.Sprint(c.port))
		if err != nil {
			fmt.Println("BinServer::thWork: net.Listen error:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Println("BinServer::thWork: Listening on port", c.port)

		for {
			conn, err := c.listener.Accept()
			if err != nil {
				fmt.Println("BinServer::thWork: Accept error:", err)
				break
			}
			go c.handleConnection(conn)
		}

	}
}

func (c *Server) handleConnection(conn net.Conn) {
	fmt.Println("BinServer::handleConnection: New client connected:", conn.RemoteAddr().String())

	// clear connections

	connectionsToRemove := make([]uint64, 0)

	c.mtx.Lock()

	for id, client := range c.clients {
		if client.IsDisconnected() {
			connectionsToRemove = append(connectionsToRemove, id)
		}
	}
	for _, id := range connectionsToRemove {
		delete(c.clients, id)
	}

	cnId := c.nextClientId
	c.nextClientId++
	c.mtx.Unlock()

	cn := NewConnectedClient(cnId, conn, c.aesKeyHex, c.onCall)

	c.mtx.Lock()
	c.clients[cnId] = cn
	c.mtx.Unlock()

	cn.Start()
}
