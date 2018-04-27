package main

import (
	"fmt"
	"github.com/varlink/go/certification/orgvarlinkcertification"
	"github.com/varlink/go/varlink"
	"os"
)

func main() {
	address := "tcp:test.varlink.org:23457"

	if len(os.Args) == 2 {
		address = os.Args[1]
	}

	c, err := varlink.NewConnection(address)
	if err != nil {
		fmt.Println("Failed to connect")
		return
	}
	defer c.Close()

	client_id, err := orgvarlinkcertification.Start().Call(c)
	if err != nil {
		fmt.Println("Start() failed")
		return
	}
	fmt.Println("Start: " + client_id)

	b, err := orgvarlinkcertification.Test01().Call(c, client_id)
	if err != nil {
		fmt.Println("Test01() failed")
		return
	}
	fmt.Printf("Test01: %t\n", b)

	i, err := orgvarlinkcertification.Test02().Call(c, client_id, b)
	if err != nil {
		fmt.Println("Test02() failed")
		return
	}
	fmt.Printf("Test02: %d\n", i)

	f, err := orgvarlinkcertification.Test03().Call(c, client_id, i)
	if err != nil {
		fmt.Println("Test03() failed")
		return
	}
	fmt.Printf("Test03: %02f\n", f)

	s, err := orgvarlinkcertification.Test04().Call(c, client_id, f)
	if err != nil {
		fmt.Println("Test04() failed")
		return
	}
	fmt.Printf("Test04: %d\n", s)
}
