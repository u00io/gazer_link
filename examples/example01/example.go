package example01

import (
	"fmt"
	"time"

	"github.com/u00io/gazer_link/gazerlink"
)

var counter int

var aesKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func thClient() {
	cl := gazerlink.NewClient(aesKey, "127.0.0.1", 3210)
	cl.Start()
	for {
		form := gazerlink.NewForm()
		form.SetFieldString("p1", "aaaa")
		resp, err := cl.Call(form, 1*time.Second)
		if err != nil {
			fmt.Println("example00: Call error:", err)
			continue
		}
		if resp.GetFieldString("p1") == "aaaa" {
			counter++
		}
	}
}

func Run() {
	aesKeyHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	srv := gazerlink.NewServer(aesKeyHex, 3210, func(form *gazerlink.Form) *gazerlink.Form {
		return form
	})
	srv.Start()

	for i := 0; i < 1000; i++ {
		fmt.Println("staring", i)
		go thClient()
		time.Sleep(10 * time.Millisecond)
	}

	for {
		time.Sleep(1000 * time.Millisecond)
		fmt.Println("example01: counter =", counter)
		counter = 0
	}
}
