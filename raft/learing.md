
# raft part A
Implement Raft leader election and heartbeats (AppendEntries RPCs with no log entries). The goal for Part 2A is for a single leader to be elected, for the leader to remain the leader if there are no failures, and for a new leader to take over if the old leader fails or if packets to/from the old leader are lost. Run go test -run 2A to test your 2A code.

在raft 2A中我们只需要实现领导选举和心跳发送这两个功能，因此只需要看论文的前5章

`Leader election`: Raft uses randomized timers to
elect leaders. This adds only a small amount of
mechanism to the heartbeats already required for any
consensus algorithm, while resolving conflicts sim-
ply and rapidly.

## Replicated state machines
raft共识的保证：

1. safety

他们保证所有非拜占庭条件下的安全性，包括网络延迟、分区、数据包丢失、重复和重新排序。

2. fully functional(available) as long as any majority

只要大多数服务器都可以运行并且可以相互通信以及与客户端通信，它们就可以完全发挥功能（可用）。

3. not depend on timing to ensure the consistency of the logs

错误的时钟和极端的消息延迟在最坏的情况下可能会导致可用性问题, 因此不是靠全局时钟保持一致性的

4. majority respond

在常见情况下，一旦集群的大多数成员响应了单轮远程过程调用，命令就可以完成；少数缓慢的服务器不会影响整体系统性能

## 设计目标

complete and practical

safe

understandability
# raft part B
Implement the leader and follower code to append new log entries, so that the go test -run 2B tests pass.
# 实现

写到这里的时候已经实现了lab2a和lab2b，所有的实现细节其实都在论文中给出了，然后下面就说下我的实现思路。

首先不管lab2a,还是lab2b,其实我们都围绕着raft的三个子问题（因为这个lab不需要我们实现成员变更）：
 1. 领导选举
 2. 日志变更
 3. 安全性
领导选举意味着我们需要个事件，这个事件中某个节点成为领导，而日志变更也意味着一个事件：领导节点发送心跳包，给其他节点。很明显的可以看到这两个事件是互斥的，因此我们只需要设置某种时间机制。
```go
rf.heartbeatTimer = time.NewTimer(HEART_BEAT_TIMEOUT * time.Millisecond)
rf.electionTimer = time.NewTimer(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
rf.applyCh = applyCh
rf.logEntries = make([]LogEntry, 1)
rf.nextIndex = make([]int, len(peers))
rf.matchIndex = make([]int, len(peers))
// 500ms and 1s
go rf.applyLogLoop()
go func() {
	for !rf.killed() {
		select {
		case <-rf.electionTimer.C:
			rf.mu.Lock()
			switch rf.peerType {
			case Follower:
				rf.switchType(Candidate)
			case Candidate:
				rf.startElection()
			}
			rf.mu.Unlock()
		case <-rf.heartbeatTimer.C:
			rf.mu.Lock()
			if rf.peerType == Leader {
				rf.leaderHeartBeats()
				rf.heartbeatTimer.Reset(HEART_BEAT_TIMEOUT * time.Millisecond)
			}
			rf.mu.Unlock()
		}
	}
}()
```
```output
# command-line-arguments
./main.go:3:1: undefined: log
./main.go:3:15: undefined: os
./main.go:5:1: undefined: fmt
./main.go:6:1: undefined: rf
./main.go:6:21: undefined: time
./main.go:6:35: undefined: HEART_BEAT_TIMEOUT
./main.go:6:56: undefined: time
./main.go:7:20: undefined: time
./main.go:7:51: undefined: HEART_BEAT_TIMEOUT
./main.go:7:74: undefined: rand
./main.go:7:74: too many errors
```

很明显可以看出这里设置了两个事件leaderHeartBeats()，和startElection()
```go
func (rf *Raft) startElection() {
	rf.currentTerm += 1
	rf.voteFor = rf.me
	rf.voteCnt = 1
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)

	for i := range rf.peers {
		if i != rf.me {
			go func(idx int) {
				rf.mu.Lock()
				args := &RequestVoteArgs{
					Term:         rf.currentTerm,
					CandidateId:  rf.me,
					LastLogIndex: len(rf.logEntries) - 1,
					LastLogTerm:  rf.logEntries[len(rf.logEntries)-1].Term,
				}
				reply := &RequestVoteReply{}
				rf.mu.Unlock()
				if rf.sendRequestVote(idx, args, reply) {
					rf.mu.Lock()
					defer rf.mu.Unlock()

					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.switchType(Follower)
						return
					}

					if reply.VoteGranted && rf.peerType == Candidate {
						rf.voteCnt++
						if rf.voteCnt > len(rf.peers)/2 {
							rf.switchType(Leader)
						}
					}
				}
			}(i)
		}
	}
}
```

在选举中给其他节点发送选举请求，当其他节点收到后按照论文描述进行回复（整个论文对这个算法描述特别清楚，因此这个实验的难度在于实际的代码语言细节）
```go
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm || (args.Term == rf.currentTerm && rf.voteFor != -1 && rf.voteFor != args.CandidateId) {
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.switchType(Follower)
	}

	lastLogIndex := len(rf.logEntries) - 1
	if rf.logEntries[lastLogIndex].Term > args.LastLogTerm || (rf.logEntries[lastLogIndex].Term == args.LastLogTerm && lastLogIndex > args.LastLogIndex) {
		reply.VoteGranted = false
		return
	}
	reply.VoteGranted = true
	rf.voteFor = args.CandidateId
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
}
```

然后心跳机制,lab b里面只是用到matchIndex和nextIndex这两个特殊的数组
```go
func (rf *Raft) leaderHeartBeats() {
	for i := range rf.peers {
		if i != rf.me {
			go rf.heartBeat(i)
		}
	}
}

func (rf *Raft) heartBeat(idx int) {
	rf.mu.Lock()
	if rf.peerType != Leader {
		rf.mu.Unlock()
		return
	}

	prevLogIndex := rf.nextIndex[idx] - 1
	entries := make([]LogEntry, len(rf.logEntries[prevLogIndex+1:]))
	copy(entries, rf.logEntries[(prevLogIndex+1):])
	// fmt.Println(idx, entries, prevLogIndex, rf.logEntries, rf.me, rf.currentTerm)
	args := &AppendEntriesArgs{
		Term:         rf.currentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  rf.logEntries[prevLogIndex].Term,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}

	reply := &AppendEntriesReply{}
	rf.mu.Unlock()
	if rf.sendRequestAppendEntries(idx, args, reply) {
		if rf.killed() {
			return
		}
		rf.mu.Lock()
		if rf.currentTerm != args.Term {
			rf.mu.Unlock()
			return
		}
		if reply.Success {
			// fmt.Println("GGGG")
			// fmt.Println(args.Entries)
			rf.matchIndex[idx] = args.PrevLogIndex + len(args.Entries)
			rf.nextIndex[idx] = rf.matchIndex[idx] + 1

			//how much log can commit
			matchLogs := make([]int, len(rf.peers))
			for i := range matchLogs {
				matchLogs[i] = rf.matchIndex[i]
			}
			sort.Ints(matchLogs)
			// fmt.Println("ccc")
			if matchLogs[len(matchLogs)/2] > rf.commitIndex {
				rf.commitIndex = matchLogs[len(matchLogs)/2]
				// fmt.Println(rf.commitIndex)
			}
		} else if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.switchType(Follower)
		} else {
			rf.nextIndex[idx] = args.PrevLogIndex
		}
		rf.mu.Unlock()
	}
}
```

收到心跳的回复：
```go
func (rf *Raft) RequestAppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// fmt.Println("111111111111111")
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	// fmt.Println(rf.me, rf.commitIndex)
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.switchType(Follower)
	}
	lastLogIndex := len(rf.logEntries) - 1
	DPrintf("%v %v\n", lastLogIndex, args.PrevLogIndex)
	if lastLogIndex < args.PrevLogIndex || rf.logEntries[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		return
	}
	reply.Success = true
	DPrintf("args:%v %v\n", rf.me, rf.logEntries)
	DPrintf("ar:%v %v %v\n", args.Entries, args.LeaderId, args.PrevLogIndex)
	if len(args.Entries) != 0 {
		rf.logEntries = rf.logEntries[:args.PrevLogIndex+1]
		rf.logEntries = append(rf.logEntries, args.Entries...)
	}
	rf.commitIndex = args.LeaderCommit
	if len(rf.logEntries)-1 < rf.commitIndex {
		rf.commitIndex = len(rf.logEntries) - 1
	}

}
```

这里part a的内容就实现完了，然后是part b还有一部分就是安全性，安全性里面规定了如何确定commitId,如果安全更新状态：
```go
func (rf *Raft) applyLogLoop() {
	for !rf.killed() {
		rf.mu.Lock()
		// fmt.Println(rf.commitIndex, rf.lastApplied)
		DPrintf("%v %v %v %v\n", rf.me, rf.lastApplied, rf.commitIndex, rf.voteFor)
		if rf.commitIndex > rf.lastApplied {
			DPrintf("%v %v\n", rf.me, rf.logEntries)
			// fmt.Println(rf.commitIndex, rf.lastApplied)
			for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
				rf.lastApplied++
				rf.applyCh <- ApplyMsg{
					CommandValid: true,
					Command:      rf.logEntries[i].Command,
					CommandIndex: i,
				}
				// fmt.Println("Ffff")
			}
			// fmt.Println(rf.commitIndex, rf.lastApplied)
		}
		rf.mu.Unlock()
		time.Sleep(HEART_BEAT_TIMEOUT * time.Millisecond)
	}
}
```

![alt text](../picture/image.png)
![alt text](<../picture/image copy.png>)

原本想详细的写写的，后面发现打字还是有点太累了。。。加上part b写完才想起要写日志，所以就处略的写了下


lab2c要做的是持久化


If a Raft-based server reboots it should resume service where it left off. This requires that Raft keep persistent state that survives a reboot. The paper's Figure 2 mentions which state should be persistent.

A real implementation would write Raft's persistent state to disk each time it changed, and would read the state from disk when restarting after a reboot. Your implementation won't use the disk; instead, it will save and restore persistent state from a Persister object (see persister.go). Whoever calls Raft.Make() supplies a Persister that initially holds Raft's most recently persisted state (if any). Raft should initialize its state from that Persister, and should use it to save its persistent state each time the state changes. Use the Persister's ReadRaftState() and SaveRaftState() methods.

整体代码量还是比较少，但是细节比较多，debug了蛮久的，这里要注意到一个paper里面提到的如果一个节点掉线了，重启后，如何快速更新日志的操作，不优化日志更新速度会在figure 8 unreliable 这个测试点超时，然后error的
```go
package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//
import (
	"bytes"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.xiuzz.com/src/labgob"
	"github.xiuzz.com/src/labrpc"
)

// import "bytes"
// import "../labgob"

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type PeerType int

const HEART_BEAT_TIMEOUT = 100

const (
	Follower PeerType = iota
	Candidate
	Leader
)

type LogEntry struct {
	Term    int
	Command interface{}
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm    int
	voteFor        int
	logEntries     []LogEntry
	applyCh        chan ApplyMsg
	peerType       PeerType
	heartbeatTimer *time.Timer
	electionTimer  *time.Timer
	voteCnt        int
	//volatile
	commitIndex int
	lastApplied int
	//volatile state on leader
	nextIndex  []int
	matchIndex []int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = rf.peerType == Leader

	// fmt.Println(isleader, rf.currentTerm, rf.me, rf.voteCnt, rf.peerType)

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// 突然意识到一个问题，持久化为什么只持久化这三个量？
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.voteFor)
	e.Encode(rf.logEntries)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var voteFor int
	var logEntries []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&voteFor) != nil ||
		d.Decode(&logEntries) != nil {
		panic("!Error when read persist")
	} else {
		rf.currentTerm = currentTerm
		rf.voteFor = voteFor
		rf.logEntries = logEntries
	}
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term        int
	CandidateId int
	//election restriction
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm || (args.Term == rf.currentTerm && rf.voteFor != -1 && rf.voteFor != args.CandidateId) {
		reply.VoteGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.switchType(Follower)
	}

	lastLogIndex := len(rf.logEntries) - 1
	if rf.logEntries[lastLogIndex].Term > args.LastLogTerm || (rf.logEntries[lastLogIndex].Term == args.LastLogTerm && lastLogIndex > args.LastLogIndex) {
		reply.VoteGranted = false
		return
	}
	reply.VoteGranted = true
	rf.voteFor = args.CandidateId
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term          int
	Success       bool
	ConflictIndex int
	ConflictTerm  int
}

func (rf *Raft) RequestAppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// fmt.Println("111111111111111")
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer rf.persist()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	// fmt.Println(rf.me, rf.commitIndex)
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.switchType(Follower)
	}
	lastLogIndex := len(rf.logEntries) - 1
	DPrintf("%v %v\n", lastLogIndex, args.PrevLogIndex)
	if lastLogIndex < args.PrevLogIndex || rf.logEntries[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		if lastLogIndex < args.PrevLogIndex {
			reply.ConflictIndex = lastLogIndex + 1
			reply.ConflictTerm = -1
		} else {
			reply.ConflictIndex = args.PrevLogIndex
			reply.ConflictTerm = rf.logEntries[args.PrevLogIndex].Term
			// 这里优化的情况是整个term不匹配，paper上面好想是没提的？ 但是不优化又过不去测试
			for i := reply.ConflictIndex - 1; i >= 1; i-- {
				if rf.logEntries[i].Term == reply.ConflictTerm {
					reply.ConflictIndex--
				} else {
					break
				}
			}
		}
		return
	}

	reply.Success = true
	DPrintf("args:%v %v\n", rf.me, rf.logEntries)
	DPrintf("ar:%v %v %v\n", args.Entries, args.LeaderId, args.PrevLogIndex)
	if len(args.Entries) != 0 {
		rf.logEntries = rf.logEntries[:args.PrevLogIndex+1]
		rf.logEntries = append(rf.logEntries, args.Entries...)
	}
	rf.commitIndex = args.LeaderCommit
	if len(rf.logEntries)-1 < rf.commitIndex {
		rf.commitIndex = len(rf.logEntries) - 1
	}

}

func (rf *Raft) sendRequestAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	// DPrintf("11111\n")
	ok := rf.peers[server].Call("Raft.RequestAppendEntries", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
// 要求Raft开始处理，将命令附加到复制的日志中。Start应该立即返回，无需等待日志追加完成
// 该服务期望你的实现为每个新提交的日志条目发送ApplyMsg到Make()的applyCh通道参数
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// 只有leader才能写入
	if rf.peerType != Leader {
		return -1, -1, false
	}

	logEntry := LogEntry{
		Command: command,
		Term:    rf.currentTerm,
	}
	rf.logEntries = append(rf.logEntries, logEntry)
	rf.persist()
	// fmt.Println(rf.logEntries)
	index = len(rf.logEntries) - 1
	term = rf.currentTerm
	rf.matchIndex[rf.me] = index
	rf.nextIndex[rf.me] = index + 1

	DPrintf("RaftNode[%d] Add Command, logIndex[%d] currentTerm[%d]", rf.me, index, term)
	return index, term, isLeader
}

func (rf *Raft) leaderHeartBeats() {
	for i := range rf.peers {
		if i != rf.me {
			go rf.heartBeat(i)
		}
	}
}

func (rf *Raft) heartBeat(idx int) {
	rf.mu.Lock()
	if rf.peerType != Leader {
		rf.mu.Unlock()
		return
	}

	prevLogIndex := rf.nextIndex[idx] - 1
	entries := make([]LogEntry, len(rf.logEntries[prevLogIndex+1:]))
	copy(entries, rf.logEntries[(prevLogIndex+1):])
	// fmt.Println(idx, entries, prevLogIndex, rf.logEntries, rf.me, rf.currentTerm)
	args := &AppendEntriesArgs{
		Term:         rf.currentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  rf.logEntries[prevLogIndex].Term,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}

	reply := &AppendEntriesReply{}
	rf.mu.Unlock()
	if rf.sendRequestAppendEntries(idx, args, reply) {
		if rf.killed() {
			return
		}
		rf.mu.Lock()
		if rf.currentTerm != args.Term {
			rf.mu.Unlock()
			return
		}
		if reply.Success {
			// fmt.Println("GGGG")
			// fmt.Println(args.Entries)
			rf.matchIndex[idx] = args.PrevLogIndex + len(args.Entries)
			rf.nextIndex[idx] = rf.matchIndex[idx] + 1

			//how much log can commit
			matchLogs := make([]int, len(rf.peers))
			for i := range matchLogs {
				matchLogs[i] = rf.matchIndex[i]
			}
			sort.Ints(matchLogs)
			// fmt.Println("ccc")
			if matchLogs[len(matchLogs)/2] > rf.commitIndex {
				rf.commitIndex = matchLogs[len(matchLogs)/2]
				// fmt.Println(rf.commitIndex)
			}
		} else if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.switchType(Follower)
			rf.persist()
		} else {
			rf.nextIndex[idx] = reply.ConflictIndex
			if reply.ConflictTerm != -1 {
				//这里实际上优化的是如果一个节点掉线了，再重启，进度落后很多的情况
				for i := args.PrevLogIndex; i >= 1; i-- {
					if rf.logEntries[i-1].Term == reply.ConflictTerm {
						rf.nextIndex[idx] = i
						break
					}
				}
			}
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) applyLogLoop() {
	for !rf.killed() {
		rf.mu.Lock()
		// fmt.Println(rf.commitIndex, rf.lastApplied)
		DPrintf("%v %v %v %v\n", rf.me, rf.lastApplied, rf.commitIndex, rf.voteFor)
		if rf.commitIndex > rf.lastApplied {
			// fmt.Println(rf.commitIndex, rf.lastApplied)
			for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
				rf.lastApplied++
				rf.applyCh <- ApplyMsg{
					CommandValid: true,
					Command:      rf.logEntries[i].Command,
					CommandIndex: i,
				}
				// fmt.Println("Ffff")
			}
			// fmt.Println(rf.commitIndex, rf.lastApplied)
		}
		rf.mu.Unlock()
		time.Sleep(HEART_BEAT_TIMEOUT * time.Millisecond)
	}
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill()  method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
// peers 一个raft对等节点（包括此节点）的网络标识符数组，用于rpc。
// me 此对等节点在peers中的索引

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	// Your initialization code here (2A, 2B, 2C).
	rf.voteFor = -1
	rf.peerType = Follower
	//300 ~ 500
	rf.heartbeatTimer = time.NewTimer(HEART_BEAT_TIMEOUT * time.Millisecond)
	rf.electionTimer = time.NewTimer(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
	rf.applyCh = applyCh
	rf.logEntries = make([]LogEntry, 1)
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	// initialize from state persisted before a crash
	rf.mu.Lock()
	rf.readPersist(persister.ReadRaftState())
	rf.mu.Unlock()
	// 500ms and 1s
	go rf.applyLogLoop()
	go func() {
		for !rf.killed() {
			select {
			case <-rf.electionTimer.C:
				rf.mu.Lock()
				switch rf.peerType {
				case Follower:
					rf.switchType(Candidate)
				case Candidate:
					rf.startElection()
				}
				rf.mu.Unlock()
			case <-rf.heartbeatTimer.C:
				rf.mu.Lock()
				if rf.peerType == Leader {
					rf.leaderHeartBeats()
					rf.heartbeatTimer.Reset(HEART_BEAT_TIMEOUT * time.Millisecond)
				}
				rf.mu.Unlock()
			}
		}
	}()
	return rf
}

func (rf *Raft) switchType(peerType PeerType) {
	if rf.peerType == peerType {
		return
	}
	rf.peerType = peerType
	switch rf.peerType {
	case Follower:
		rf.heartbeatTimer.Stop()
		rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)
		rf.voteFor = -1
	case Candidate:
		rf.startElection()
	case Leader:
		for i := range rf.nextIndex {
			rf.nextIndex[i] = len(rf.logEntries)
		}
		for i := range rf.matchIndex {
			rf.matchIndex[i] = 0
		}

		rf.electionTimer.Stop()
		rf.leaderHeartBeats()
		rf.heartbeatTimer.Reset(HEART_BEAT_TIMEOUT * time.Millisecond)
	}
}

func (rf *Raft) startElection() {
	rf.currentTerm += 1
	rf.voteFor = rf.me
	rf.voteCnt = 1
	rf.persist()
	rf.electionTimer.Reset(time.Duration((3*HEART_BEAT_TIMEOUT + 2*rand.Intn(HEART_BEAT_TIMEOUT))) * time.Millisecond)

	for i := range rf.peers {
		if i != rf.me {
			go func(idx int) {
				rf.mu.Lock()
				args := &RequestVoteArgs{
					Term:         rf.currentTerm,
					CandidateId:  rf.me,
					LastLogIndex: len(rf.logEntries) - 1,
					LastLogTerm:  rf.logEntries[len(rf.logEntries)-1].Term,
				}
				reply := &RequestVoteReply{}
				rf.mu.Unlock()
				if rf.sendRequestVote(idx, args, reply) {
					rf.mu.Lock()
					defer rf.mu.Unlock()

					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.switchType(Follower)
						rf.persist()
						return
					}

					if reply.VoteGranted && rf.peerType == Candidate {
						rf.voteCnt++
						if rf.voteCnt > len(rf.peers)/2 {
							rf.switchType(Leader)
						}
					}
				}
			}(i)
		}
	}
}

```

![alt text](<../picture/image copy 2.png>)
