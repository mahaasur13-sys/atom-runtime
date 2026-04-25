// Package consensus — ATOM-037 Deterministic Consensus Engine tests.
package consensus

import (
	"testing"
)

func TestNewConsensusNode(t *testing.T) {
	node := NewConsensusNode("n1")
	if node.ID != "n1" {
		t.Fatalf("expected ID n1, got %s", node.ID)
	}
	if node.State != StateFollower {
		t.Fatalf("expected initial state Follower, got %v", node.State)
	}
	if node.Term != 0 {
		t.Fatalf("expected initial term 0, got %d", node.Term)
	}
}

func TestElectLeader_Deterministic(t *testing.T) {
	nodes := []ConsensusNode{
		{ID: "n1", Term: 2, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
		{ID: "n2", Term: 3, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
		{ID: "n3", Term: 3, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}, {TraceID: "t2", Seq: 1}}}}, // longest log
	}

	leader := ElectLeader(nodes)
	if leader != "n3" {
		t.Fatalf("expected n3 (highest term=3, longest log), got %s", leader)
	}

	// Deterministic: same nodes must produce same leader
	leader2 := ElectLeader(nodes)
	if leader2 != leader {
		t.Fatalf("ElectLeader not deterministic: got %s then %s", leader, leader2)
	}
}

func TestElectLeader_TermTieBreak(t *testing.T) {
	nodes := []ConsensusNode{
		{ID: "n1", Term: 1, Store: &EventStore{log: []Event{}}},
		{ID: "n2", Term: 1, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
	}
	leader := ElectLeader(nodes)
	if leader != "n2" {
		t.Fatalf("expected n2 (same term, longer log), got %s", leader)
	}
}

func TestEventStoreAppend_SeqViolation(t *testing.T) {
	store := NewEventStore()
	e := Event{TraceID: "t1", Seq: 1, PrevHash: "", EventHash: "h1"}

	if err := store.Append(e); err != nil {
		t.Fatalf("first append should succeed: %v", err)
	}

	// Seq violation: expect 2, got 5
	e2 := Event{TraceID: "t1", Seq: 5, PrevHash: "h1", EventHash: "h2"}
	if err := store.Append(e2); err == nil {
		t.Fatal("expected seq violation error, got nil")
	}
}

func TestEventStoreAppend_HashViolation(t *testing.T) {
	store := NewEventStore()
	e := Event{TraceID: "t1", Seq: 1, PrevHash: "", EventHash: "h1"}

	if err := store.Append(e); err != nil {
		t.Fatalf("first append should succeed: %v", err)
	}

	// Hash violation: expect prevHash="h1", got "wrong"
	e2 := Event{TraceID: "t1", Seq: 2, PrevHash: "wrong", EventHash: "h2"}
	if err := store.Append(e2); err == nil {
		t.Fatal("expected hash violation error, got nil")
	}
}

func TestEventStoreAppend_MultipleTraces(t *testing.T) {
	store := NewEventStore()
	events := []Event{
		{"t1", 1, "", "h1"},
		{"t1", 2, "h1", "h2"},
		{"t2", 1, "", "h3"},
		{"t1", 3, "h2", "h4"},
		{"t2", 2, "h3", "h5"},
	}

	for _, e := range events {
		if err := store.Append(e); err != nil {
			t.Fatalf("append failed for %+v: %v", e, err)
		}
	}

	if len(store.log) != 5 {
		t.Fatalf("expected 5 events, got %d", len(store.log))
	}
	if store.lastSeq["t1"] != 3 {
		t.Fatalf("expected lastSeq[t1]=3, got %d", store.lastSeq["t1"])
	}
	if store.lastSeq["t2"] != 2 {
		t.Fatalf("expected lastSeq[t2]=2, got %d", store.lastSeq["t2"])
	}
}

func TestQuorumReached(t *testing.T) {
	// 3 nodes with term > 0
	nodes := []ConsensusNode{
		{ID: "n1", Term: 1, State: StateLeader},
		{ID: "n2", Term: 1, State: StateLeader},
		{ID: "n3", Term: 1, State: StateLeader},
		{ID: "n4", Term: 0, State: StateFollower},
		{ID: "n5", Term: 0, State: StateFollower},
	}

	// GEB_THRESHOLD(5) = 3 — only 3 nodes with Term > 0
	if !QuorumReached(nodes, 1, GEB_THRESHOLD(5)) {
		t.Fatal("quorum should be reached with 3/5 active")
	}
	if QuorumReached(nodes, 1, 4) {
		t.Fatal("quorum should NOT be reached with threshold 4")
	}
}

func TestCheckSingleSourceOfTruth(t *testing.T) {
	// Identical logs
	nodes1 := []ConsensusNode{
		{ID: "n1", Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
		{ID: "n2", Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
	}
	if !CheckSingleSourceOfTruth(nodes1) {
		t.Fatal("identical logs should pass")
	}

	// Different length
	nodes2 := []ConsensusNode{
		{ID: "n1", Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
		{ID: "n2", Store: &EventStore{log: []Event{}}},
	}
	if CheckSingleSourceOfTruth(nodes2) {
		t.Fatal("different length logs should fail")
	}

	// Single node
	nodes3 := []ConsensusNode{
		{ID: "n1", Store: &EventStore{log: []Event{}}},
	}
	if !CheckSingleSourceOfTruth(nodes3) {
		t.Fatal("single node should always pass")
	}
}

func TestCheckNoSplitBrain(t *testing.T) {
	// One leader
	nodes1 := []ConsensusNode{
		{ID: "n1", State: StateLeader},
		{ID: "n2", State: StateFollower},
		{ID: "n3", State: StateFollower},
	}
	if !CheckNoSplitBrain(nodes1) {
		t.Fatal("single leader should pass")
	}

	// Two leaders
	nodes2 := []ConsensusNode{
		{ID: "n1", State: StateLeader},
		{ID: "n2", State: StateLeader},
		{ID: "n3", State: StateFollower},
	}
	if CheckNoSplitBrain(nodes2) {
		t.Fatal("two leaders should fail")
	}
}

func TestGEBAdvanceTerm(t *testing.T) {
	node := NewConsensusNode("n1")
	if node.Term != 0 {
		t.Fatalf("initial term should be 0, got %d", node.Term)
	}
	newTerm := GEBAdvanceTerm(node)
	if newTerm != 1 {
		t.Fatalf("expected term 1, got %d", newTerm)
	}
	if node.State != StateCandidate {
		t.Fatalf("expected state Candidate, got %v", node.State)
	}

	// Advance again
	newTerm2 := GEBAdvanceTerm(node)
	if newTerm2 != 2 {
		t.Fatalf("expected term 2, got %d", newTerm2)
	}
}

func TestCommitEntry_QuorumRequired(t *testing.T) {
	e := Event{TraceID: "t1", Seq: 1, PrevHash: "", EventHash: "h1"}

	// No active nodes (all Term=0) — should fail
	nodes1 := []ConsensusNode{
		{ID: "n1", Term: 0, State: StateFollower},
		{ID: "n2", Term: 0, State: StateFollower},
		{ID: "n3", Term: 0, State: StateFollower},
	}
	commitNode1 := &ConsensusNode{ID: "c", Term: 0, Store: NewEventStore()}
	if CommitEntry(commitNode1, e, nodes1, 1) {
		t.Fatal("commit should fail with 0 active nodes")
	}

	// 2 active nodes, threshold=2, quorum reached, commit succeeds
	commitNode2 := &ConsensusNode{ID: "c", Term: 1, State: StateLeader, Store: NewEventStore()}
	nodes2 := []ConsensusNode{
		{ID: "n1", Term: 0, Store: nil},
		{ID: "n2", Term: 1, State: StateLeader, Store: NewEventStore()},
		{ID: "n3", Term: 1, State: StateLeader, Store: NewEventStore()},
	}
	if !CommitEntry(commitNode2, e, nodes2, 1) {
		t.Fatal("commit should succeed with quorum")
	}
}

func TestHandleMessage_APPEND(t *testing.T) {
	node := NewConsensusNode("n2")
	node.State = StateFollower
	node.Term = 5

	msg := Message{
		Type:      "APPEND",
		Term:      6,
		FromID:    "n1",
		TraceID:   "t1",
		Seq:       1,
		EventHash: "h1",
		// PrevHash intentionally "" for first event in trace
	}

	reply, _ := HandleMessage(node, msg)
	if reply == nil {
		t.Fatal("expected ACK reply")
	}
	if reply.Type != "ACK" {
		t.Fatalf("expected ACK, got %s", reply.Type)
	}
	if node.Term != 6 {
		t.Fatalf("expected term 6, got %d", node.Term)
	}
	if node.State != StateFollower {
		t.Fatalf("expected state Follower, got %v", node.State)
	}
}

func TestHandleMessage_VOTE_REQUEST(t *testing.T) {
	node := NewConsensusNode("n2")
	node.Term = 5

	msg := Message{
		Type:   "VOTE_REQUEST",
		Term:   6,
		FromID: "n1",
	}

	reply, _ := HandleMessage(node, msg)
	if reply == nil {
		t.Fatal("expected VOTE reply")
	}
	if reply.Type != "VOTE" {
		t.Fatalf("expected VOTE, got %s", reply.Type)
	}
	if node.VotedFor != "n1" {
		t.Fatalf("expected voted for n1, got %s", node.VotedFor)
	}
}

func TestDeterminism_SameNodesSameLeader(t *testing.T) {
	for round := 0; round < 10; round++ {
		nodes := []ConsensusNode{
			{ID: "n1", Term: 2, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}},
			{ID: "n2", Term: 3, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}, {TraceID: "t2", Seq: 1}, {TraceID: "t3", Seq: 1}}}}, // 3 events
			{ID: "n3", Term: 3, Store: &EventStore{log: []Event{{TraceID: "t1", Seq: 1}}}}, // 1 event, shorter
		}
		leader := ElectLeader(nodes)
		if leader != "n2" {
			t.Fatalf("round %d: expected n2 (term=3, longest log=3), got %s", round, leader)
		}
	}
}
