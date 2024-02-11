package main

import (
	"fmt"
	"sync"
	"time"
)

var wx sync.WaitGroup

func main() {
	c := make(chan bool)
	wx.Add(1)
	go func() {
		time.Sleep(1 * time.Second)
		tmp := <-c
		fmt.Printf("%v\n", tmp)
		fmt.Println("finished")
		wx.Done()
	}()
	start := time.Now()
	time.Sleep(3 * time.Second)
	c <- false
	fmt.Printf("send took %v\n", time.Since(start))
	wx.Wait()
}
