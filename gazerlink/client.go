package gazerlink

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

type Request struct {
	outForm *Form
	inForm  *Form
	err     error
	done    bool
}

type Client struct {
	aesKeyHex string
	aesKey    []byte
	mtx       sync.Mutex
	conn      net.Conn
	ipAddr    string
	port      int

	transactionCounter uint64

	requests map[string]*Request
}

func NewClient(aesKeyHex string, ipAddr string, port int) *Client {
	var c Client
	c.aesKeyHex = aesKeyHex
	c.aesKey, _ = hex.DecodeString(aesKeyHex)
	c.ipAddr = ipAddr
	c.port = port
	c.requests = make(map[string]*Request)
	return &c
}

func (c *Client) Start() {
	go c.thWork()
}

func (c *Client) Stop() {

}

func (c *Client) thWork() {
	var conn net.Conn
	var err error
	inputBuffer := make([]byte, 4*1024*1024)
	inputBufferIndex := 0
	connectionInfo := c.ipAddr + ":" + fmt.Sprint(c.port)
	for {
		c.mtx.Lock()
		conn = c.conn
		c.mtx.Unlock()

		if conn == nil {
			// try to connect
			conn, err = net.Dial("tcp", c.ipAddr+":"+fmt.Sprint(c.port))
			if err != nil || conn == nil {
				inputBufferIndex = 0
				fmt.Println("Client::thWork: connect error:", err)
				time.Sleep(1000 * time.Millisecond)
				continue
			}
			c.mtx.Lock()
			c.conn = conn
			c.mtx.Unlock()
		}

		buffer := make([]byte, 4096)
		var n int
		n, err = conn.Read(buffer)
		if err != nil {
			fmt.Println("Client::thWork: read error:", err)
			c.mtx.Lock()
			c.conn.Close()
			c.conn = nil
			c.mtx.Unlock()
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if inputBufferIndex+n > len(inputBuffer) {
			break
		}

		copy(inputBuffer[inputBufferIndex:], buffer[0:n])
		inputBufferIndex += n

		// Parse frame
		if inputBufferIndex < 4 {
			continue
		}

		frameLength := int(binary.LittleEndian.Uint32(inputBuffer[0:4]))
		if frameLength > 1024*1024 {
			break
		}
		if inputBufferIndex < frameLength {
			continue
		}

		if frameLength < 4 {
			fmt.Println("Client::thWork: invalid frame length:", frameLength)
			break
		}

		frameData := inputBuffer[4:frameLength]
		decryptedFrameData, err := DecryptAESGCM(frameData, c.aesKey)
		if err != nil {
			fmt.Println("Client::thWork: DecryptAESGCM error:", err)
			break
		}
		form, err := ParseForm(decryptedFrameData)
		if err != nil {
			break
		}
		go c.ProcessFrame(form)
		copy(inputBuffer[0:], inputBuffer[frameLength:inputBufferIndex])
		inputBufferIndex -= frameLength
	}
	fmt.Println("Client::thWork disconnected from", connectionInfo)
	c.mtx.Lock()
	c.conn.Close()
	c.conn = nil
	c.mtx.Unlock()
	fmt.Println("Client::thWork stopped for", connectionInfo)
}

func (c *Client) ProcessFrame(form *Form) {
	transactionId := form.GetFieldString("_transaction_id")
	c.mtx.Lock()
	req, ok := c.requests[transactionId]
	if ok {
		req.inForm = form
		req.done = true
	}
	c.mtx.Unlock()
}

func (c *Client) Call(form *Form, timeout time.Duration) (*Form, error) {
	var err error
	c.mtx.Lock()
	transactionId := fmt.Sprint(c.transactionCounter)
	conn := c.conn
	c.transactionCounter++
	c.mtx.Unlock()

	form.SetFieldString("_transaction_id", transactionId)

	dtBeginWaitConnection := time.Now()
	for conn == nil {
		time.Sleep(10 * time.Millisecond)
		c.mtx.Lock()
		conn = c.conn
		c.mtx.Unlock()
		if time.Since(dtBeginWaitConnection) > timeout {
			return nil, fmt.Errorf("no connection")
		}
	}

	formData := form.Serialize()
	formData, err = EncryptAESGCM(formData, c.aesKey)
	if err != nil {
		return nil, err
	}
	frameLength := uint32(len(formData) + 4)
	frameBuffer := make([]byte, frameLength)
	binary.LittleEndian.PutUint32(frameBuffer[0:4], frameLength)
	copy(frameBuffer[4:], formData)

	var req Request
	req.outForm = form
	req.inForm = nil
	req.err = nil
	req.done = false

	c.mtx.Lock()
	c.requests[transactionId] = &req
	sent := 0
	for sent < int(frameLength) {
		var n int
		n, err = conn.Write(frameBuffer[sent:])
		if err != nil {
			req.err = err
			req.done = true
			break
		}
		sent += n
	}
	c.mtx.Unlock()

	if err != nil {
		return nil, err
	}

	// Wait for response
	if timeout > 0 {
		startTime := time.Now()
		for {
			c.mtx.Lock()
			if req.done {
				c.mtx.Unlock()
				break
			}
			c.mtx.Unlock()
			if time.Since(startTime) > timeout {
				c.mtx.Lock()
				delete(c.requests, transactionId)
				c.mtx.Unlock()
				return nil, fmt.Errorf("timeout")
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	c.mtx.Lock()
	delete(c.requests, transactionId)
	c.mtx.Unlock()

	return req.inForm, req.err
}
