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

	err = orgvarlinkcertification.Start(c, 0)
	if err != nil {
		fmt.Println("Start() failed")
		return
	}

	var client_id string
	_, err = orgvarlinkcertification.ReadStart_(c, &client_id)
	if err != nil {
		fmt.Println("StartRead() failed")
		return
	}
	fmt.Println("Start: " + client_id)

	err = orgvarlinkcertification.Test01(c, 0, client_id)
	if err != nil {
		fmt.Println("Test01() failed")
		return
	}

	var flag bool
	_, err = orgvarlinkcertification.ReadTest01_(c, &flag)
	if err != nil {
		fmt.Println("ReadTest01() failed")
		return
	}
	fmt.Printf("Test01: %t\n", flag)
}
