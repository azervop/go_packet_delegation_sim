package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"math/rand"
)

func main() {
	args := os.Args[1:]
	arrRate, e := strconv.ParseFloat(args[0], 64)
	if e != nil {
		panic(e)
	}
	depRate, e := strconv.ParseFloat(args[1], 64)
	if e != nil {
		panic(e)
	}
	N, e := strconv.Atoi(args[2])
	if e != nil {
		panic(e)
	}

	queue := []float64{}
	busy := false
	served := 0
	arrTime := rand.ExpFloat64()
	depTime := math.Inf(1)

	custTime := 0.0
	avgQueueDelay := 0.0
	avgDelay := 0.0

	t := 0.0
	for {
		t = min(arrTime, depTime)
		if arrTime < depTime {
			// handle arrival
			if busy {
				// enqueue customer
				queue = append(queue, arrTime)
			} else {
				// become busy
				custTime = t
				busy = true
				depTime = t + rand.ExpFloat64()/depRate
			}
			arrTime = t + rand.ExpFloat64()/arrRate
		} else {
			// handle departure

			// track statistics
			served++
			avgDelay += t - custTime
			if served >= N {
				break
			}
			if len(queue) > 0 {
				// pop customer
				custTime, queue = queue[0], queue[1:]
				avgQueueDelay += t - custTime
				depTime = t + rand.ExpFloat64()/depRate
			} else {
				// go idle
				busy = false
				depTime = math.Inf(1)
			}
		}
	}

	fmt.Println("Total time taken:", t, "=> N/t", float64(N)/t)
	fmt.Println("Avg queue delay:", avgQueueDelay/float64(N))
	fmt.Println("Avg delay:", avgDelay/float64(N))
}
