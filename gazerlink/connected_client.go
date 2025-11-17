package gazerlink

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
)

type ConnectedClient struct {
	onCall    func(*Form) *Form
	mtx       sync.Mutex
	id        uint64
	conn      net.Conn
	aesKeyHex string
	aesKey    []byte
}

func NewConnectedClient(id uint64, conn net.Conn, aesKeyHex string, onCall func(*Form) *Form) *ConnectedClient {
	var c ConnectedClient
	c.id = id
	c.conn = conn
	c.aesKeyHex = aesKeyHex
	c.aesKey, _ = hex.DecodeString(aesKeyHex)
	c.onCall = onCall
	return &c
}

func (c *ConnectedClient) IsDisconnected() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.conn == nil
}

func (c *ConnectedClient) Start() {
	go c.thWork()
}

func (c *ConnectedClient) thWork() {
	c.mtx.Lock()
	conn := c.conn
	c.mtx.Unlock()

	if conn == nil {
		return
	}

	connectionInfo := conn.RemoteAddr().String()

	fmt.Println("ConnectedClient::thWork started for", connectionInfo)
	inputBuffer := make([]byte, 100*1024)
	inputBufferIndex := 0
	for {
		c.mtx.Lock()
		conn := c.conn
		c.mtx.Unlock()

		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			break
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
		if inputBufferIndex < frameLength {
			continue
		}

		frameData := inputBuffer[4:frameLength]
		decryptedFrameData, err := DecryptAESGCM(frameData, c.aesKey)
		if err != nil {
			break
		}
		form, err := ParseForm(decryptedFrameData)
		if err != nil {
			break
		}
		c.ProcessFrame(form)

		// Shift remaining data to the beginning of the buffer
		copy(inputBuffer[0:], inputBuffer[frameLength:inputBufferIndex])
		inputBufferIndex -= frameLength

	}

	c.mtx.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mtx.Unlock()

	fmt.Println("ConnectedClient::thWork stopped for", connectionInfo)
}

func (c *ConnectedClient) ProcessFrame(form *Form) {
	responseForm := c.onCall(form)
	responseForm.SetFieldString("_transaction_id", form.GetFieldString("_transaction_id"))
	c.SendForm(responseForm)
}

func (c *ConnectedClient) SendForm(form *Form) error {
	c.mtx.Lock()
	formData := form.Serialize()
	formData, err := EncryptAESGCM(formData, c.aesKey)
	if err != nil {
		c.mtx.Unlock()
		return err
	}

	frameLength := uint32(len(formData) + 4)
	frameBuffer := make([]byte, frameLength)
	binary.LittleEndian.PutUint32(frameBuffer[0:4], frameLength)
	copy(frameBuffer[4:], formData)
	sent := 0
	for sent < int(frameLength) {
		n, err := c.conn.Write(frameBuffer[sent:])
		if err != nil {
			c.mtx.Unlock()
			return err
		}
		sent += n
	}

	c.mtx.Unlock()
	return nil
}
