package main

import "fmt"

func main() {
	c := make(chan bool, 1)
	go func() {
		fmt.Println("test")
	}()
	c <- true
}
