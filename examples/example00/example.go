package example00

import (
	"fmt"
	"time"

	"github.com/u00io/gazer_link/gazerlink"
)

func OnCall(form *gazerlink.Form) *gazerlink.Form {
	val := form.GetFieldString("p1")
	fmt.Println("example00: OnCall received p1 =", val)
	return form
}

func Run() {
	aesKeyHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	srv := gazerlink.NewServer(aesKeyHex, 3210, OnCall)
	srv.Start()

	time.Sleep(500 * time.Millisecond)

	cl := gazerlink.NewClient(aesKeyHex, "127.0.0.1", 3210)
	cl.Start()

	for {
		time.Sleep(100 * time.Millisecond)

		form := gazerlink.NewForm()
		form.SetFieldString("p1", "aaaa")
		resp, err := cl.Call(form, 1*time.Second)
		if err != nil {
			fmt.Println("example00: Call error:", err)
			continue
		}
		fmt.Println("example00: Call succeeded", resp.GetFieldString("p1"))
	}
}
