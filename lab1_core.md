因为官方不让上传该实验的代码故把核心代码写在markdown里面保存下：
```go
//master
package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"sync"
	"time"
)

// Task Type
type TaskType int

const (
	Type_Map TaskType = iota
	Type_Reduce
)

type TaskState int

const (
	Waiting TaskState = iota
	Running
	finished
)

type GlobalState int

const (
	Global_Map GlobalState = iota
	Global_Reduce
	Global_Done
)

type Task struct {
	TID       int
	Type      TaskType
	State     TaskState
	Input     []string
	ReduceTh  int // only for reduce
	RSzie     int
	StartTime time.Time
}

type Master struct {
	// Your definitions here.
	State      GlobalState
	TaskCnt    int
	MapTask    chan *Task
	ReduceTask chan *Task
	TaskMap    map[int]*Task
	MapSize    int
	ReduceSize int
}

// sync lock
var mu sync.Mutex

// Your code here -- RPC handlers for the worker to call.
func (m *Master) AssginTask(args *TaskArgs, reply *TaskReply) error {
	mu.Lock()
	defer mu.Unlock()
	fmt.Println("AssginTask ......")
	//core to switch m/r/f and process workers
	switch m.State {
	case Global_Map:
		fmt.Println("map..........")
		if len(m.MapTask) > 0 {
			task := <-m.MapTask
			task.State = Running
			task.StartTime = time.Now()
			m.TaskMap[task.TID] = task
			reply.CanDo = Do
			reply.Task = *task
		} else {
			reply.CanDo = NoWork
			//check can switch to next phase
			if m.checkMap() {
				m.nextPhase()
			}
		}
	case Global_Reduce:
		fmt.Println("Reduce..........")
		if len(m.ReduceTask) > 0 {
			task := <-m.ReduceTask
			task.State = Running
			task.StartTime = time.Now()
			m.TaskMap[task.TID] = task
			reply.CanDo = Do
			reply.Task = *task
		} else {
			reply.CanDo = NoWork
			//check can switch to next phase
			if m.checkReduce() {
				m.nextPhase()
			}
		}
	case Global_Done:
		fmt.Println("Done..........")
		reply.CanDo = Exit
	default:
		panic("The phase undefined!")
	}

	return nil
}

func (m *Master) ReciveHeart(args *WorkStateArgs, reply *WorkStateReply) error {
	if args.WorkHeart == WorkDone {
		if m.TaskMap[args.TID] != nil {
			m.TaskMap[args.TID].State = finished
		}
	}
	return nil
}
func (m *Master) checkMap() bool {
	ans := true
	for i := 0; i < m.TaskCnt; i++ {
		if m.TaskMap[i] == nil {
			ans = false
			break
		}
		if m.TaskMap[i].State != finished {
			ans = false
			break
		}
	}
	fmt.Printf("this map ans: %v\n", ans)
	return ans
}

func (m *Master) checkReduce() bool {
	ans := true
	for i := m.MapSize; i < m.TaskCnt; i++ {
		if m.TaskMap[i] == nil {
			ans = false
			break
		}
		if m.TaskMap[i].State != finished {
			ans = false
			break
		}
	}
	fmt.Printf("this reduce ans: %v\n", ans)
	return ans
}

func (m *Master) nextPhase() {
	fmt.Printf("finished this phase : %v\n", m.State)
	if m.State == Global_Map {
		m.initReduce()
		m.State = Global_Reduce
	} else if m.State == Global_Reduce {
		m.State = Global_Done
	}
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
func (m *Master) Done() bool {
	mu.Lock()
	defer mu.Unlock()
	ret := m.State == Global_Done
	return ret
}

// create TID
func (m *Master) CreateTID() int {
	res := m.TaskCnt
	m.TaskCnt++
	return res
}

func (m *Master) initMap(files []string) {
	fmt.Println("map task init.......")
	for _, file := range files {
		mapTask := Task{
			TID:   m.CreateTID(),
			State: Waiting,
			Type:  Type_Map,
			Input: []string{file},
			RSzie: m.ReduceSize,
		}
		fmt.Printf("make a map task: %v\n", mapTask.TID)
		m.MapTask <- &mapTask
	}
}

func (m *Master) initReduce() {
	fmt.Println("reduce task init.......")
	dir, _ := os.Getwd()
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
	}
	for i := 0; i < m.ReduceSize; i++ {
		// mr-X-Y
		input := []string{}
		for _, file := range files {
			pattern := fmt.Sprintf("^mr-.+-%v$", i)
			matched, _ := regexp.MatchString(pattern, file.Name())
			if matched {
				input = append(input, file.Name())
			}
		}
		fmt.Println(input)
		reduceTask := Task{
			TID:      m.CreateTID(),
			Type:     Type_Reduce,
			State:    Waiting,
			Input:    input,
			ReduceTh: i,
			RSzie:    m.ReduceSize,
		}
		fmt.Printf("make a reduce task: %v\n", reduceTask.ReduceTh)
		m.ReduceTask <- &reduceTask
	}
}

// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{
		State:      Global_Map,
		TaskCnt:    0,
		MapTask:    make(chan *Task, len(files)),
		ReduceTask: make(chan *Task, nReduce),
		TaskMap:    make(map[int]*Task, len(files)+nReduce),
		ReduceSize: nReduce,
	}

	// Your code here.
	m.initMap(files)
	m.server()
	m.CrashHandle()
	return &m
}

func (m *Master) CrashHandle() {
	for {
		time.Sleep(time.Second * 2)
		mu.Lock()
		if m.State == Global_Done {
			mu.Unlock()
			break
		}
		for _, task := range m.TaskMap {
			if task.State == Running && time.Since(task.StartTime) >= 10*time.Second {
				fmt.Printf("is crash : %v\n", task.TID)
				task.State = Waiting
				if task.Type == Type_Map {
					m.MapTask <- task
				} else {
					m.ReduceTask <- task
				}
				delete(m.TaskMap, task.TID)
			}
		}
		mu.Unlock()
	}
}

```

```go
//woker
package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}
type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	flag := true
	for flag {
		args := TaskArgs{}
		reply := TaskReply{}
		call("Master.AssginTask", &args, &reply)
		switch reply.CanDo {
		case Exit:
			flag = false
		case NoWork:
			fmt.Println("NoWork.....")
			time.Sleep(time.Second)
		case Do:
			task := &reply.Task
			fmt.Println(reply)
			if task.Type == Type_Map {
				doMapTask(mapf, task)
				CallFinined(task.TID)
			} else if task.Type == Type_Reduce {
				doReduceTask(reducef, task)
				CallFinined(task.TID)
			}
		default:
			panic("bad args!")
		}
	}
}

func doMapTask(mapf func(string, string) []KeyValue, task *Task) {
	intermediate := []KeyValue{}
	for _, filename := range task.Input {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		context, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		file.Close()
		kva := mapf(filename, string(context))
		intermediate = append(intermediate, kva...)
	}

	//write into tmp file mr-x-y
	for i := 0; i < task.RSzie; i++ {
		filename := fmt.Sprintf("mr-%v-%v", task.TID, i)
		tmpfile, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(filename)
		encoder := json.NewEncoder(tmpfile)
		for _, kv := range intermediate {
			if ihash(kv.Key)%task.RSzie == i {
				encoder.Encode(kv)
			}
		}
		tmpfile.Close()
	}
}

func doReduceTask(reducef func(string, []string) string, task *Task) {
	fmt.Println("this is reduce")
	intermediate := shuffe(task.Input)
	dir, _ := os.Getwd()
	tmpfile, err := os.CreateTemp(dir, "mr-out-tmpfile-")
	if err != nil {
		log.Fatal(err)
	}
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(tmpfile, "%v %v\n", intermediate[i].Key, output)
		i = j
	}
	oname := fmt.Sprintf("mr-out-%v", task.ReduceTh)
	tmpfile.Close()
	os.Rename(tmpfile.Name(), oname)
}

func shuffe(files []string) []KeyValue {
	kva := []KeyValue{}
	for _, file := range files {
		tmpfile, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		decoder := json.NewDecoder(tmpfile)
		for {
			var kv KeyValue
			err := decoder.Decode(&kv)
			if err != nil { //eof
				break
			}
			kva = append(kva, kv)
		}
		tmpfile.Close()
	}

	sort.Sort(ByKey(kva))
	return kva
}
func CallFinined(TID int) {
	args := WorkStateArgs{WorkDone, TID}
	reply := WorkStateReply{}
	call("Master.ReciveHeart", &args, &reply)
}

// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Master.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
```

```go
//rpc
package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//
type CallState int 

const (
	Exit CallState = iota 
	NoWork 
	Do
)

type WorkState int 

const (
	Working WorkState = iota
	WorkDone
)
type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
type TaskArgs struct {
}

type TaskReply struct {
	CanDo CallState
	Task Task
}

type WorkStateArgs struct {
	WorkHeart WorkState
	TID int 
}

type WorkStateReply struct {}
// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}

```