package main

import "time"
import "sync"

var done bool
var mu sync.Mutex

func main() {
	time.Sleep(1 * time.Second)
	println("start")
	go periodic()
	time.Sleep(5 * time.Second)
	mu.Lock()
	done = true
	mu.Unlock()
	println("cancelled")
	time.Sleep(3 * time.Second)
}

func periodic() {
	for {
		println("tick")
		time.Sleep(1 * time.Second)
		mu.Lock()
		if done {
			mu.Unlock()
			return
		}
		mu.Unlock()
	}
}
