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
	fmt.Printf("Start: '%v'\n", client_id)

	b1, err := orgvarlinkcertification.Test01().Call(c, client_id)
	if err != nil {
		fmt.Println("Test01() failed")
		return
	}
	fmt.Printf("Test01: '%v'\n", b1)

	i2, err := orgvarlinkcertification.Test02().Call(c, client_id, b1)
	if err != nil {
		fmt.Println("Test02() failed")
		return
	}
	fmt.Printf("Test02: '%v'\n", i2)

	f3, err := orgvarlinkcertification.Test03().Call(c, client_id, i2)
	if err != nil {
		fmt.Println("Test03() failed")
		return
	}
	fmt.Printf("Test03: '%v'\n", f3)

	s4, err := orgvarlinkcertification.Test04().Call(c, client_id, f3)
	if err != nil {
		fmt.Println("Test04() failed")
		return
	}
	fmt.Printf("Test04: '%v'\n", s4)

	b5, i5, f5, s5, err := orgvarlinkcertification.Test05().Call(c, client_id, s4)
	if err != nil {
		fmt.Println("Test05() failed")
		return
	}
	fmt.Printf("Test05: '%v'\n", b5)

	o6, err := orgvarlinkcertification.Test06().Call(c, client_id, b5, i5, f5, s5)
	if err != nil {
		fmt.Println("Test06() failed")
		return
	}
	fmt.Printf("Test06: '%v'\n", o6)

	m7, err := orgvarlinkcertification.Test07().Call(c, client_id, o6)
	if err != nil {
		fmt.Println("Test07() failed")
		return
	}
	fmt.Printf("Test07: '%v'\n", m7)

	m8, err := orgvarlinkcertification.Test08().Call(c, client_id, m7)
	if err != nil {
		fmt.Println("Test08() failed")
		return
	}
	fmt.Printf("Test08: '%v'\n", m8)

	t9, err := orgvarlinkcertification.Test09().Call(c, client_id, m8)
	if err != nil {
		fmt.Println("Test09() failed")
		return
	}
	fmt.Printf("Test09: '%v'\n", t9)

	receive10, err := orgvarlinkcertification.Test10().Send(c, varlink.More, client_id, t9)
	if err != nil {
		fmt.Println("Test10() failed")
		return
	}

	fmt.Println("Test10() Send:")
	var a10 []string
	for {
		s10, flags10, err := receive10()
		if err != nil {
			fmt.Println("Test10() receive failed")
			return
		}
		a10 = append(a10, s10)
		fmt.Printf("  Receive: '%v'\n", s10)

		if flags10&varlink.Continues == 0 {
			break
		}
	}
	fmt.Printf("Test10: '%v'\n", a10)

	_, err = orgvarlinkcertification.Test11().Send(c, varlink.Oneway, client_id, a10)
	if err != nil {
		fmt.Println("Test11() failed")
		return
	}
	fmt.Println("Test11: ''")

	end, err := orgvarlinkcertification.End().Call(c, client_id)
	if err != nil {
		fmt.Println("End() failed")
		return
	}
	fmt.Printf("End: '%v'\n", end)
}
