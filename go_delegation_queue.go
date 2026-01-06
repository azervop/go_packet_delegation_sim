package main

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

var CSV_FOLDER string = "csv"
var eventLogFile *os.File

type simulationParameters struct {
	id              int     // Simulation ID
	arrivalRate     float64 // λ
	departureRate   float64 // μ
	N               int     // Number of messages to process
	keepProbability float64 // Probability of keeping a message in the queue
	theta           int     // Threshold for delegation
}

func unwrapParamJson(params map[string]interface{}) []simulationParameters {
	// Expand parameter JSON with arrays into a flat list of simulationParameters.
	// Accepts a map where values may be scalars or arrays and returns the
	// Cartesian product of parameters as a slice of simulationParameters.
	paramList := []simulationParameters{}
	// Helper function to recursively generate all combinations
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}

	var recur func(idx int, current map[string]interface{})
	recur = func(idx int, current map[string]interface{}) {
		if idx == len(keys) {
			// Build simulationParameters from current
			param := simulationParameters{}
			for k, v := range current {
				switch k {
				case "arrivalRate":
					switch vv := v.(type) {
					case float64:
						param.arrivalRate = vv
					case int:
						param.arrivalRate = float64(vv)
					}
				case "departureRate":
					switch vv := v.(type) {
					case float64:
						param.departureRate = vv
					case int:
						param.departureRate = float64(vv)
					}
				case "N":
					switch vv := v.(type) {
					case float64:
						param.N = int(vv)
					case int:
						param.N = vv
					}
				case "keepProbability":
					switch vv := v.(type) {
					case float64:
						param.keepProbability = vv
					case int:
						param.keepProbability = float64(vv)
					}
				case "theta":
					switch vv := v.(type) {
					case float64:
						param.theta = int(vv)
					case int:
						param.theta = vv
					}
				}
			}
			paramList = append(paramList, param)
			return
		}

		key := keys[idx]
		val := params[key]
		switch vv := val.(type) {
		case []interface{}:
			for _, elem := range vv {
				current[key] = elem
				recur(idx+1, current)
			}
		default:
			current[key] = vv
			recur(idx+1, current)
		}
	}

	recur(0, make(map[string]interface{}))
	return paramList
}

type message struct {
	id            int
	priority      int
	arrivalTime   float64
	dequeueTime   float64
	departureTime float64
}

// messagePriorityQueue implements a min-heap ordered by priority then arrival time.
// Lower `priority` value means higher precedence; ties broken by `arrivalTime`.
// NOTE: priority is not currently being used in the simulation logic.
type messagePriorityQueue []*message

// Implement heap.Interface for messagePriorityQueue
func (pq messagePriorityQueue) Len() int { return len(pq) }

func (pq messagePriorityQueue) Less(i, j int) bool {
	if pq[i].priority == pq[j].priority {
		return pq[i].arrivalTime < pq[j].arrivalTime
	}
	return pq[i].priority < pq[j].priority
}

func (pq messagePriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *messagePriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*message))
}

func (pq *messagePriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

type pqueue struct {
	messages          messagePriorityQueue
	processingMessage *message
	busy              bool
	arrivalRate       float64
	processingRate    float64
	nextArrival       float64
	nextDeparture     float64
	theta             int
	keepProbability   float64
	lastChange        float64
	numProcessed      int
	maxProcessed      int
	totalDelay        float64
	averageState      float64
}

// pqueue is the manager queue that can delegate messages to a downstream VNF.
// It uses a priority heap, tracks service state, and accumulates statistics.

type queue struct {
	messages       []message
	busy           bool
	arrivalRate    float64
	processingRate float64
	nextArrival    float64
	nextDeparture  float64
	lastChange     float64
	numProcessed   int
	maxProcessed   int
	totalDelay     float64
	averageState   float64
}

// queue is a simple FIFO VNF queue implemented with a slice of messages.
// It maintains timing and statistical counters used by the simulation loop.

func logExperimentParameters(params []simulationParameters) {
	// Ensure the CSV folder exists before writing
	err := os.MkdirAll(CSV_FOLDER, 0755)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(filepath.Join(CSV_FOLDER, "params.csv"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "id,arr_rate,dep_rate,N,theta,k\n")
	for i, p := range params {
		fmt.Fprintf(f, "%d,%.2f,%.2f,%d,%d,%.2f\n", i, p.arrivalRate, p.departureRate, p.N, p.theta, p.keepProbability)
	}
	fmt.Println("Experiment parameters logged to", filepath.Join(CSV_FOLDER, "params.csv"))
}

// logExperimentParameters writes experiment parameters to CSV for later analysis.

func logEvent(id int, time float64, node string, event string, msgID int) {
	if eventLogFile != nil {
		fmt.Fprintf(eventLogFile, "%d,%.6f,%s,%s,%d\n", id, time, node, event, msgID)
	}
}

// logEvent appends a single simulation event to the global event CSV file.

func mgrQueueArrive(q *pqueue, msgID *int, t float64, nextQueue *queue, simID int) {
	// handle arrival into the manager queue. If idle, start service; otherwise
	// push into the priority heap. Update averages and schedule next arrival.
	newMsg := message{id: *msgID, arrivalTime: t, priority: 0}
	// logEvent(simID, t, "manager", "arrival", *msgID)
	(*msgID)++
	// if len(nextQueue.messages) >= q.theta {
	// 	if rand.Float64() >= q.keepProbability {
	// 		newMsg.priority = 1 // high priority
	// 	}
	// }
	q.averageState += float64(len(q.messages)) * (t - q.lastChange)
	if !q.busy {
		// become busy
		q.busy = true
		q.processingMessage = &newMsg
		newMsg.dequeueTime = t
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	} else {
		heap.Push(&q.messages, &newMsg)
	}
	q.lastChange = t
	q.nextArrival = t + rand.ExpFloat64()/q.arrivalRate
}

func mgrQueueProcess(q *pqueue, t float64, nextQueue *queue, simID int) {
	// handle departure from the manager queue. Update statistics, decide whether
	// to delegate or forward to the VNF, and start service for the next message.
	q.processingMessage.departureTime = t
	q.numProcessed++
	q.totalDelay += q.processingMessage.departureTime - q.processingMessage.arrivalTime
	q.averageState += float64(len(q.messages)+1) * (t - q.lastChange)
	q.lastChange = t

	processed := q.processingMessage
	if len(nextQueue.messages) >= q.theta {
		if rand.Float64() >= q.keepProbability {
			logEvent(simID, t, "manager", "delegation", processed.id)
		} else {
			// logEvent(simID, t, "manager", "departure", processed.id)
			queueArrive(nextQueue, &processed.id, t, simID)
		}
	} else {
		// logEvent(simID, t, "manager", "departure", processed.id)
		queueArrive(nextQueue, &processed.id, t, simID)
	}

	// if processed.priority == 1 {
	// 	// Delegated packet - not forwarded to VNF
	// } else {
	// 	// Forwarded packet - send to VNF
	// }

	if len(q.messages) == 0 {
		q.busy = false
		q.nextDeparture = math.Inf(1)
		return
	}

	nextMsg := heap.Pop(&q.messages)
	q.processingMessage = nextMsg.(*message)
	q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
}

func queueArrive(q *queue, msgID *int, t float64, simID int) {
	// Arrival into the VNF queue: log, enqueue, and possibly start service.
	logEvent(simID, t, "vnf", "arrival", *msgID)
	newMsg := message{id: *msgID, arrivalTime: t}
	q.messages = append(q.messages, newMsg)
	q.averageState += float64(len(q.messages)-1) * (t - q.lastChange)
	if !q.busy {
		// become busy
		q.busy = true
		newMsg.dequeueTime = t
		q.nextDeparture = t + rand.ExpFloat64()/q.processingRate
	}
	q.lastChange = t
}

func queueProcess(q *queue, t float64, simID int) {
	// Process departure from the VNF queue, update statistics, and schedule
	// the next departure or set the queue idle.
	q.messages[0].departureTime = t
	q.numProcessed++
	q.totalDelay += q.messages[0].departureTime - q.messages[0].arrivalTime
	q.averageState += float64(len(q.messages)) * (t - q.lastChange)
	q.lastChange = t
	logEvent(simID, t, "vnf", "departure", q.messages[0].id)
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

func runSimulation(params simulationParameters) {
	// runSimulation executes a single simulation with the provided parameters.
	// It advances the event loop until the target number of processed items
	// (`params.N`) is reached and prints a summary on completion.
	if params.keepProbability*params.arrivalRate >= params.departureRate {
		panic(fmt.Sprintf("Unstable system: keepProbability (%.2f) * arrivalRate (%.2f) >= processingRate (%.2f)",
			params.keepProbability, params.arrivalRate, params.departureRate))
	}

	mgrQueue := pqueue{
		messages:          messagePriorityQueue{},
		processingMessage: nil,
		busy:              false,
		arrivalRate:       params.arrivalRate,
		processingRate:    8 * params.departureRate,
		nextArrival:       rand.ExpFloat64() / params.arrivalRate,
		nextDeparture:     math.Inf(1),
		theta:             params.theta,
		keepProbability:   params.keepProbability,
		lastChange:        0,
		numProcessed:      0,
		maxProcessed:      params.N,
		totalDelay:        0,
		averageState:      0,
	}
	vnfQueue := queue{
		messages:       []message{},
		busy:           false,
		arrivalRate:    params.arrivalRate,
		processingRate: params.departureRate,
		nextArrival:    math.Inf(1),
		nextDeparture:  math.Inf(1),
		lastChange:     0,
		numProcessed:   0,
		maxProcessed:   params.N,
		totalDelay:     0,
		averageState:   0,
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
				mgrQueueArrive(&mgrQueue, &msgID, t, &vnfQueue, params.id)
			} else {
				mgrQueueProcess(&mgrQueue, t, &vnfQueue, params.id)
			}
		} else if queueType == "vnf" {
			if vnfQueue.numProcessed >= vnfQueue.maxProcessed {
				break
			}

			if t == vnfQueue.nextDeparture {
				queueProcess(&vnfQueue, t, params.id)
			}
		}
	}
	fmt.Printf("Simulation %d completed: Total time %.6f, Throughput %.6f\n",
		params.id, t, float64(params.N)/t)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run go_delegation_queue.go param_file.json")
		return
	}
	paramFile := os.Args[1]
	jsonFile, err := os.ReadFile(paramFile)
	if err != nil {
		fmt.Println("Error reading parameter file:", err)
		return
	}

	// parse parameter file as map
	var paramMap map[string]interface{}
	err = json.Unmarshal(jsonFile, &paramMap)
	if err != nil {
		fmt.Println("Error parsing parameter file:", err)
		return
	}

	subFolder := fmt.Sprintf("k=%.01f_theta=%d",
		float64(paramMap["keepProbability"].(float64)),
		int(paramMap["theta"].(float64)))
	CSV_FOLDER = filepath.Join(CSV_FOLDER, subFolder)

	// Ensure the CSV folder exists before writing
	err = os.MkdirAll(CSV_FOLDER, 0755)
	if err != nil {
		fmt.Println("Error creating CSV folder:", err)
		return
	}
	paramList := unwrapParamJson(paramMap)
	if len(paramList) == 0 {
		fmt.Println("No valid parameters found in the file.")
		return
	}
	fmt.Println(paramList)

	logExperimentParameters(paramList)

	// Open event log file
	var errLog error
	eventLogFile, errLog = os.Create(filepath.Join(CSV_FOLDER, "events.csv"))
	if errLog != nil {
		fmt.Println("Error creating event log file:", errLog)
		return
	}
	defer eventLogFile.Close()
	fmt.Fprintf(eventLogFile, "id,time,node,event,msg_id\n")

	// Run simulations
	for i, param := range paramList {
		fmt.Printf("Running simulation %d with parameters: %+v\n", i, param)
		param.id = i
		runSimulation(param)
	}

	fmt.Println("Event log saved to", filepath.Join(CSV_FOLDER, "events.csv"))
}
