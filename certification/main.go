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

	flag, err := orgvarlinkcertification.Test01().Call(c, client_id)
	if err != nil {
		fmt.Println("Test01() failed")
		return
	}
	fmt.Printf("Test01: %t\n", flag)
}
