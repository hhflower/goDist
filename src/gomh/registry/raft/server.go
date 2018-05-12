package raft

import (
	"encoding/json"
	"fmt"
	"net"
	//	"golang.org/x/net/context"
	pb "gomh/registry/raft/proto"
	"gomh/util"
	"google.golang.org/grpc"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"
)

type server struct {
	*eventDispatcher

	mutex   sync.RWMutex
	stopped chan bool

	name  string
	path  string
	state string

	currentLeader string
	currentTerm   uint64
	confPath      string

	transporter    Transporter
	log            *Log
	conf           *Config
	peers          map[string]*Peer
	voteGrantedNum int
	votedForTerm   uint64 // vote one peer as a leader in curterm

	leaderAcceptTime   int64
	heartbeatInterval  int64
	appendEntryRespCnt int
}

type Server interface {
	Start() error
	IsRunning() bool
	State() string
	CanCommitLog() bool

	AddPeer(name string, connectionInfo string) error
	RemovePeer(name string) error
}

func NewServer(name, path, confPath string) (Server, error) {
	s := &server{
		name:              name,
		path:              path,
		confPath:          confPath,
		state:             Stopped,
		log:               newLog(),
		heartbeatInterval: 1000, // 1000ms
	}
	return s, nil
}

func (s *server) SetTerm(term uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.currentTerm = term
}

func (s *server) IncrAppendEntryResp() {
	s.appendEntryRespCnt += 1
}

func (s *server) QuorumSize() int {
	return len(s.peers)/2 + 1
}

func (s *server) CanCommitLog() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.appendEntryRespCnt >= s.QuorumSize()
}

func (s *server) IsRunning() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, ok := RunningStates[s.state]
	return ok
}

func (s *server) State() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.state
}

func (s *server) VotedForTerm() uint64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.votedForTerm
}

func (s *server) SetVotedForTerm(term uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.votedForTerm = term
}

func (s *server) VoteGrantedNum() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.voteGrantedNum
}

func (s *server) IncrVoteGrantedNum() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.voteGrantedNum += 1
}

func (s *server) IncrTermForvote() {
	s.currentTerm += 1
}

func (s *server) SetState(state string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.state = state
}

func (s *server) VoteForSelf() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.voteGrantedNum = 1 // vote for itself
	s.peers[s.conf.Host].SetVoteRequestState(VoteGranted)
}

// Init steps:
// check if running or initiated before
// load configuration file
// load raft log
// recover server persistent status
// set state = Initiated
func (s *server) Init() error {
	if s.IsRunning() {
		return fmt.Errorf("server has been running with state:%d", s.State())
	}

	if s.State() == Initiated {
		s.SetState(Initiated)
		return nil
	}

	err := os.Mkdir(path.Join(s.path, "snapshot"), 0600)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("raft initiation error: %s", err)
	}

	err = s.loadConf()
	if err != nil {
		return fmt.Errorf("raft load config error: %s", err)
	}

	logpath := path.Join(s.path, "internlog")
	err = os.Mkdir(logpath, 0600)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("raft-log initiation error: %s", err)
	}
	if err = s.log.LogInit(fmt.Sprintf("%s/%s%s", logpath, s.conf.LogPrefix, s.conf.CandidateName)); err != nil {
		return fmt.Errorf("raft-log initiation error: %s", err)
	}
	fmt.Printf("%+v\n", s.log.entries)

	err = s.LoadState()
	if err != nil {
		return fmt.Errorf("raft load srvstate error: %s", err)
	}

	s.SetState(Initiated)
	return nil
}

// start steps:
// comlete initiation
// set state = Follower
// new goroutine for tcp listening
// enter loop with a propriate state
func (s *server) Start() error {
	if s.IsRunning() {
		return fmt.Errorf("server has been running with state:%d", s.State())
	}

	if err := s.Init(); err != nil {
		return err
	}

	s.stopped = make(chan bool)

	s.SetState(Follower)

	loopch := make(chan int)
	go func() {
		defer func() { loopch <- 1 }()
		s.ListenAndServe()
	}()
	s.loop()
	return nil
}

func (s *server) ListenAndServe() {
	server := grpc.NewServer()
	pb.RegisterRequestVoteServer(server, &RequestVoteImp{server: s})
	pb.RegisterAppendEntriesServer(server, &AppendEntriesImp{server: s})
	//pb.RegisterHeartbeatServer(server, &SendHeartbeatImp{server: s})

	fmt.Printf("To listen on %s\n", s.conf.Host)
	address, err := net.Listen("tcp", s.conf.Host)
	if err != nil {
		panic(err)
	}

	if err := server.Serve(address); err != nil {
		panic(err)
	}
}

func (s *server) loadConf() error {
	confpath := path.Join(s.path, "raft.cfg")
	if s.confPath != "" {
		confpath = path.Join(s.path, s.confPath)
	}

	cfg, err := ioutil.ReadFile(confpath)
	if err != nil {
		fmt.Errorf("open config file failed, err:%s", err)
		return nil
	}

	conf := &Config{}
	if err = json.Unmarshal(cfg, conf); err != nil {
		return err
	}
	s.conf = conf
	s.peers = make(map[string]*Peer)
	for _, c := range s.conf.PeerHosts {
		s.peers[c] = &Peer{
			Name:   c,
			Host:   c,
			server: s,
		}
	}

	return nil
}

func (s *server) loop() {
	for s.State() != Stopped {
		fmt.Printf("current state:%s, term:%d\n", s.State(), s.currentTerm)
		switch s.State() {
		case Follower:
			s.followerLoop()
		case Candidate:
			s.candidateLoop()
		case Leader:
			s.leaderLoop()
			//		case Snapshotting:
			//			s.snapshotLoop()
		case Stopped:
			// TODO: do something before server stop
			break
		}
	}
}

func (s *server) candidateLoop() {
	t := time.NewTimer(time.Duration(150+rand.Intn(150)) * time.Millisecond)
	for s.State() == Candidate {
		select {
		case <-t.C:
			if s.State() != Candidate {
				return
			}
			s.currentTerm += 1 // candidate term to increased by 1
			s.VoteForSelf()
			lindex, lterm := s.log.LastLogInfo()
			for idx, _ := range s.peers {
				if s.conf.Host == s.peers[idx].Host {
					continue
				}
				s.peers[idx].RequestVoteMe(lindex, lterm)
			}
			if s.VoteGrantedNum() >= s.QuorumSize() {
				s.SetState(Leader)
			} else {
				t.Reset(time.Duration(150+rand.Intn(150)) * time.Millisecond)
			}

		case isStop := <-s.stopped:
			if isStop {
				s.SetState(Stopped)
				break
			}
		}
	}
}

func (s *server) followerLoop() {
	t := time.NewTimer(time.Duration(s.heartbeatInterval) * time.Millisecond)
	for s.State() == Follower {
		select {
		case <-t.C:
			if s.State() != Follower {
				return
			}
			if util.GetTimestampInMilli()-s.leaderAcceptTime > s.heartbeatInterval*2 {
				s.IncrTermForvote()
				s.SetState(Candidate)
			} else {
				s.FlushState()
			}
			t.Reset(time.Duration(s.heartbeatInterval) * time.Millisecond)
		case isStop := <-s.stopped:
			if isStop {
				s.SetState(Stopped)
				break
			}
		}
	}
}

func (s *server) leaderLoop() {
	// to request append entry as a new leader is elected
	s.appendEntryRespCnt = 1
	lindex, lterm := s.log.LastLogInfo()
	entry := &pb.LogEntry{
		Index:       lindex + 1,
		Term:        s.currentTerm,
		Commandname: "nop",
		Command:     []byte(""),
	}
	s.log.AppendEntry(&LogEntry{Entry: entry})

	for idx, _ := range s.peers {
		if s.conf.Host == s.peers[idx].Host {
			continue
		}
		s.peers[idx].RequestAppendEntries([]*pb.LogEntry{entry}, lindex, lterm)
	}
	if s.CanCommitLog() {
		index, _ := s.log.LastLogInfo()
		s.log.UpdateCommitIndex(index)
		s.FlushState()
		fmt.Printf("to commit log, index:%d term:%d\n", index, s.currentTerm)
	}

	// send heartbeat as leader state
	t := time.NewTimer(time.Duration(s.heartbeatInterval) * time.Millisecond)
	for s.State() == Leader {
		select {
		case <-t.C:
			if s.State() != Leader {
				return
			}
			if util.GetTimestampInMilli()-s.leaderAcceptTime > s.heartbeatInterval {
				s.appendEntryRespCnt = 1
				for idx, _ := range s.peers {
					if s.conf.Host == s.peers[idx].Host {
						continue
					}
					s.peers[idx].RequestAppendEntries([]*pb.LogEntry{}, 0, 0)
				}
				if !s.CanCommitLog() {
					s.SetState(Candidate)
				}
			}
			if s.State() == Leader {
				t.Reset(time.Duration(s.heartbeatInterval) * time.Millisecond)
			}
		case isStop := <-s.stopped:
			if isStop {
				s.SetState(Stopped)
				break
			}
		}
	}
}

func (s *server) AddPeer(name string, connectionInfo string) error {
	if s.peers[name] != nil {
		return nil
	}

	if s.name != name {
		ti := time.Duration(s.heartbeatInterval) * time.Millisecond
		peer := NewPeer(s, name, connectionInfo, ti)

		s.peers[peer.Name] = peer
	}

	return nil
}

func (s *server) RemovePeer(name string) error {
	if name == s.name {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	peer := s.peers[name]
	if peer == nil {
		return nil
	}

	delete(s.peers, name)
	return nil
}
