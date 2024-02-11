package main

import (
	"sync"
)

var wx sync.WaitGroup
var mu sync.Mutex

func main() {
	counter := 0
	wx.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			mu.Lock()
			defer mu.Unlock()
			counter++
			wx.Done()
		}()
	}
	wx.Wait()
	println(counter)
}
