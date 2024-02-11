package main

import (
	"fmt"
	"runtime"
	"sync"
)

func main() {
	runtime.GOMAXPROCS(1)
	fmt.Println("begin")
	var a int = 0
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			if a == 1 {
				fmt.Println("a == 1")
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			a = 1
		}
	}()
	wg.Wait()
}
