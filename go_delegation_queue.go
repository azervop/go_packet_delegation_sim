package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"math/rand"
)

type message struct {
	id int
	arrivalTime float64
	dequeueTime float64
	departureTime float64
}

type queue struct {
	messages []message
	busy bool
	arrivalRate float64
	processingRate float64
	nextArrival float64
	nextDeparture float64
	lastChange float64
	numProcessed int
	maxProcessed int
	totalDelay float64
	averageState float64
}

func mgrQueueArrive(q *queue, msgID *int, t float64) {
	// handle arrival
	newMsg := message{id: *msgID, arrivalTime: t}
	(*msgID)++
	q.messages = append(q.messages, newMsg)
	q.averageState += float64(len(q.messages)-1) * (t - q.lastChange)
	if !q.busy {
		// become busy
		q.busy = true
		newMsg.dequeueTime = t
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	}
	q.lastChange = t
	q.nextArrival = t + rand.ExpFloat64()/q.arrivalRate
}

func mgrQueueProcess(q *queue, t float64, nextQueue *queue) {
	// handle departure
	// track statistics
	q.messages[0].departureTime = t
	q.numProcessed++
	q.totalDelay += q.messages[0].departureTime - q.messages[0].arrivalTime
	q.averageState += float64(len(q.messages)) * (t - q.lastChange)
	q.lastChange = t
	// if q.numProcessed >= q.maxProcessed {
	// 	return
	// }
	processed := q.messages[0]
	queueArrive(nextQueue, &processed.id, t)
	q.messages = q.messages[1:]
	if len(q.messages) > 0 {
		// pop customer
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	} else {
		// go idle
		q.busy = false
		q.nextDeparture = math.Inf(1)
	}
}

func queueArrive(q *queue, msgID *int, t float64) {
	// fmt.Println("Time:", t, "=> New message arrived at VNF")
	// handle arrival
	newMsg := message{id: *msgID, arrivalTime: t}
	// (*msgID)++
	q.messages = append(q.messages, newMsg)
	q.averageState += float64(len(q.messages)-1) * (t - q.lastChange)
	if !q.busy {
		// become busy
		q.busy = true
		newMsg.dequeueTime = t
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	}
	q.lastChange = t
	q.nextArrival = t + rand.ExpFloat64()/q.arrivalRate
}

func queueProcess(q *queue, t float64) {
	// handle departure
	// track statistics
	q.messages[0].departureTime = t
	q.numProcessed++
	q.totalDelay += q.messages[0].departureTime - q.messages[0].arrivalTime
	q.averageState += float64(len(q.messages)) * (t - q.lastChange)
	q.lastChange = t
	if q.numProcessed >= q.maxProcessed {
		return
	}
	processed := q.messages[0]
	q.messages = q.messages[1:]
	if len(q.messages) > 0 {
		// pop customer
		processed.dequeueTime = t
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	} else {
		// go idle
		q.busy = false
		q.nextDeparture = math.Inf(1)
	}
}

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

	mgrQueue := queue{
		messages:    []message{},
		busy:        false,
		arrivalRate: arrRate,
		processingRate: depRate,
		nextArrival:  rand.ExpFloat64() / arrRate,
		nextDeparture: math.Inf(1),
		lastChange:   0,
		numProcessed: 0,
		maxProcessed: N,
		totalDelay:   0,
		averageState: 0,
	}
	vnfQueue := queue{
		messages:     []message{},
		busy:         false,
		arrivalRate:  arrRate,
		processingRate: depRate,
		nextArrival:  math.Inf(1),
		nextDeparture: math.Inf(1),
		lastChange:   0,
		numProcessed: 0,
		maxProcessed: N,
		totalDelay:   0,
		averageState: 0,
	}
	msgID := 0

	t := 0.0
	for {
		queueType := "manager"
		t = min(mgrQueue.nextArrival, mgrQueue.nextDeparture)
		if vnfQueue.nextDeparture < t {
			queueType = "vnf"
			t = vnfQueue.nextDeparture
		}

		if queueType == "manager" {
			if t == mgrQueue.nextArrival {
				// fmt.Println("Time:", t, "=> New message arrived at manager")
				mgrQueueArrive(&mgrQueue, &msgID, t)
			} else {
				// fmt.Println("Time:", t, "=> Processing message at manager")
				mgrQueueProcess(&mgrQueue, t, &vnfQueue)
			}
		} else if queueType == "vnf" {
			if vnfQueue.numProcessed >= vnfQueue.maxProcessed {
				break
			}

			if t == vnfQueue.nextArrival {
				// queueArrive(&vnfQueue, &msgID, t)
			} else {
				// fmt.Println("Time:", t, "=> Processing message at VNF")
				queueProcess(&vnfQueue, t)
			}
		}
	}


	fmt.Println("Total time taken:", t, "=> N/t", float64(N)/t)
	fmt.Println("Avg delay:", vnfQueue.totalDelay/float64(N), "(theory:", 1/(vnfQueue.processingRate - vnfQueue.arrivalRate), ")")
	fmt.Println("Avg state:", vnfQueue.averageState/t, " (theory:", vnfQueue.arrivalRate/(vnfQueue.processingRate - vnfQueue.arrivalRate), ")")
}
