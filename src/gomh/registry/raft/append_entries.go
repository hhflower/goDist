package raft

import (
	"fmt"
	"golang.org/x/net/context"
	pb "gomh/registry/raft/proto"
	"gomh/util"
	"google.golang.org/grpc"
	"sync"
)

type AppendEntriesImp struct {
	server *server
	mutex  sync.Mutex
}

func (e *AppendEntriesImp) AppendEntries(ctx context.Context, req *pb.AppendEntriesReuqest) (*pb.AppendEntriesResponse, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	pb := &pb.AppendEntriesResponse{
		Success: false,
	}
	if req.GetTerm() >= e.server.currentTerm {
		e.server.SetState(Follower)
		e.server.currentTerm = req.GetTerm()
		e.server.currentLeader = req.GetLeaderName()
		e.server.leaderAcceptTime = util.GetTimestampInMilli()

		lindex, _ := e.server.log.LastLogInfo()
		entries := req.GetEntries()
		if req.GetPreLogIndex() > lindex {
			pb.Success = false
			pb.Index = lindex
		} else if req.GetPreLogIndex() < lindex {
			backindex := len(e.server.log.entries) - 1
			for i := backindex; i >= 0; i-- {
				if e.server.log.entries[i].Entry.GetIndex() <= req.GetPreLogIndex() {
					backindex = i
					break
				}
			}
			e.server.log.entries = e.server.log.entries[0 : backindex+1]
			e.server.log.RefreshLog()
			if e.server.log.entries[backindex].Entry.GetIndex() == req.GetPreLogIndex() {
				for _, entry := range entries {
					e.server.log.AppendEntry(&LogEntry{Entry: entry})
				}
				pb.Success = true
			} else {
				pb.Index = e.server.log.entries[backindex].Entry.GetIndex()
				pb.Success = false
			}
		} else if req.GetPreLogIndex() == lindex {
			for _, entry := range entries {
				e.server.log.AppendEntry(&LogEntry{Entry: entry})
			}
			pb.Success = true
		}
	}

	if pb.Success {
		e.server.log.UpdateCommitIndex(req.GetCommitIndex())
	}
	lindex, lterm := e.server.log.LastLogInfo()
	pb.Index = lindex
	pb.Term = lterm

	fmt.Printf("to be follower to %s\n", req.LeaderName)
	return pb, nil
}

func RequestAppendEntriesCli(s *server, peer *Peer, entries []*pb.LogEntry, lindex, lterm uint64) {
	if s.State() != Leader {
		fmt.Println("only leader can request append entries.")
		return
	}

	conn, err := grpc.Dial(peer.Host, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("dail rpc failed, err: %s\n", err)
		return
	}

	client := pb.NewAppendEntriesClient(conn)

	req := &pb.AppendEntriesReuqest{
		Term:        s.currentTerm,
		PreLogIndex: lindex,
		PreLogTerm:  lterm,
		CommitIndex: s.log.CommitIndex(),
		LeaderName:  s.conf.Host,
		Entries:     entries,
	}

	res, err := client.AppendEntries(context.Background(), req)

	if err != nil {
		fmt.Printf("leader reqeust AppendEntries failed, err:%s\n", err)
		return
	}
	fmt.Printf("[appendentry]from:%s to:%s rpcRes:%+v\n", s.conf.Host, peer.Host, res)

	if res.Success {
		s.IncrAppendEntryResp()
	} else {
		el := []*pb.LogEntry{}
		for _, e := range s.log.entries {
			if e.Entry.GetIndex() <= res.Index {
				continue
			}
			el = append(el, e.Entry)
		}
		req := &pb.AppendEntriesReuqest{
			Term:        s.currentTerm,
			PreLogIndex: res.Index,
			PreLogTerm:  res.Term,
			CommitIndex: s.log.CommitIndex(),
			LeaderName:  s.conf.Host,
			Entries:     el,
		}

		res, err = client.AppendEntries(context.Background(), req)

		if err != nil {
			fmt.Printf("leader reqeust AppendEntries failed, err:%s\n", err)
			return
		} else {
			fmt.Printf("synlog res: %+v %t %d %d\n", req, res.Success, res.Index, res.Term)
		}
		if res.Success {
			s.IncrAppendEntryResp()
		}
	}

	//TODO
}
