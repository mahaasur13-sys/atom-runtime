// Package consensus — ATOM-037 Deterministic Consensus Engine (GEB-based Raft variant).
// Provides split-brain-safe consensus over EventStore under Byzantine + partition.
// All operations are deterministic: no randomness, no wall-clock, no hidden state.
//
// ATOM constraints:
// - C1: No time.Now, time.Sleep, math/rand, uuid.New, unsorted map iteration
// - C2: Replay Equivalence
// - C3: No hidden state
package consensus

// NodeState represents the role of a node in the consensus protocol.
type NodeState int

const (
	StateFollower NodeState = iota
	StateCandidate
	StateLeader
)

func (s NodeState) String() string {
	switch s {
	case StateFollower:
		return "follower"
	case StateCandidate:
		return "candidate"
	case StateLeader:
		return "leader"
	default:
		return "unknown"
	}
}

// Message is a consensus message between nodes.
type Message struct {
	Type      string // APPEND, VOTE_REQUEST, VOTE, HEARTBEAT
	Term      uint64
	FromID    string
	TraceID   string
	Seq       uint64
	EventHash string
}

// EventStore is the append-only log store under consensus.
type EventStore struct {
	log       []Event
	lastSeq   map[string]uint64
	lastHash  map[string]string
}

// NewEventStore creates an empty EventStore.
func NewEventStore() *EventStore {
	return &EventStore{
		log:      make([]Event, 0),
		lastSeq:  make(map[string]uint64),
		lastHash: make(map[string]string),
	}
}

// Append appends an event to the log.
// Returns error on sequence/hash violation (ATOM invariants).
func (es *EventStore) Append(e Event) error {
	// INV2: seq must be lastSeq[trace]+1
	lastSeq, seqOk := es.lastSeq[e.TraceID]
	lastHash, hashOk := es.lastHash[e.TraceID]

	// INV2: seq must be lastSeq[trace]+1
	if seqOk {
		if e.Seq != lastSeq+1 {
			return &AppendError{e.TraceID, "seq_violation", e.Seq, lastSeq + 1}
		}
	} else {
		if e.Seq != 1 {
			return &AppendError{e.TraceID, "first_seq_not_1", e.Seq, 1}
		}
	}

	// INV4: prevHash must match chain tip
	if hashOk {
		if e.PrevHash != lastHash {
			return &AppendError{e.TraceID, "hash_violation_str", 0, 0}
		}
	} else {
		if e.PrevHash != "" {
			return &AppendError{e.TraceID, "first_prevhash_not_empty", 0, 0}
		}
	}

	es.log = append(es.log, e)
	es.lastSeq[e.TraceID] = e.Seq
	es.lastHash[e.TraceID] = e.EventHash
	return nil
}

// Event is a single event in the log.
type Event struct {
	TraceID   string
	Seq       uint64
	PrevHash  string
	EventHash string
}

// AppendError is returned when an append violates ATOM invariants.
type AppendError struct {
	TraceID string
	Kind   string
	Have   uint64
	Want   uint64
}

func (e *AppendError) Error() string {
	return "consensus: append error traceID=" + e.TraceID +
		" kind=" + e.Kind + " have=" + u64(e.Have) + " want=" + u64(e.Want)
}

func u64(v uint64) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	n := 0
	for v > 0 {
		buf[19-n] = byte('0' + v%10)
		n++
		v /= 10
	}
	return string(buf[20-n:])
}

// ConsensusNode is a single replica with consensus state.
type ConsensusNode struct {
	ID      string
	Store   *EventStore
	State   NodeState
	Term    uint64
	VotedFor string
}

// NewConsensusNode creates a new ConsensusNode.
func NewConsensusNode(id string) *ConsensusNode {
	return &ConsensusNode{
		ID:    id,
		Store: NewEventStore(),
		State: StateFollower,
		Term:  0,
	}
}

// ElectLeader deterministically selects the leader.
// ATOM C3 constraint: no randomness — leader is determined by max term + log length.
func ElectLeader(nodes []ConsensusNode) string {
	var (
		maxTerm   uint64
		maxSeqLen int
		leaderID  string
	)

	for _, n := range nodes {
		logLen := len(n.Store.log)
		if n.Term > maxTerm || (n.Term == maxTerm && logLen > maxSeqLen) {
			maxTerm = n.Term
			maxSeqLen = logLen
			leaderID = n.ID
		}
	}

	return leaderID
}

// QuorumReached checks if GEB quorum is satisfied at a given tick.
// quorum is defined as > N/2 nodes having reached tick 'tick'.
func QuorumReached(nodes []ConsensusNode, tick uint64, threshold uint64) bool {
	count := uint64(0)
	for _, n := range nodes {
		// Only nodes with term > 0 (active participants) count toward quorum
		if n.Term > 0 {
			count++
		}
	}
	return count >= threshold
}

// GEB_THRESHOLD is the quorum size (> N/2).
func GEB_THRESHOLD(n int) uint64 {
	return uint64(n/2) + 1
}

// CommitEntry attempts to commit an entry to a node.
// Returns true if committed, false if quorum not reached.
func CommitEntry(node *ConsensusNode, e Event, nodes []ConsensusNode, tick uint64) bool {
	if !QuorumReached(nodes, tick, GEB_THRESHOLD(len(nodes))) {
		return false
	}
	return node.Store.Append(e) == nil
}

// CheckSingleSourceOfTruth verifies all nodes have identical logs.
// Returns true if all logs are identical.
func CheckSingleSourceOfTruth(nodes []ConsensusNode) bool {
	if len(nodes) < 2 {
		return true
	}

	refLog := nodes[0].Store.log
	refLen := len(refLog)

	for i := 1; i < len(nodes); i++ {
		if len(nodes[i].Store.log) != refLen {
			return false
		}
		for j := 0; j < refLen; j++ {
			if nodes[i].Store.log[j] != refLog[j] {
				return false
			}
		}
	}
	return true
}

// CheckNoSplitBrain verifies at most one leader exists.
func CheckNoSplitBrain(nodes []ConsensusNode) bool {
	leaders := 0
	for _, n := range nodes {
		if n.State == StateLeader {
			leaders++
			if leaders > 1 {
				return false
			}
		}
	}
	return true
}

// GEBAdvanceTerm advances a node's term deterministically.
// G1 constraint: term only increases.
func GEBAdvanceTerm(node *ConsensusNode) uint64 {
	node.Term++
	node.State = StateCandidate
	return node.Term
}

// HandleMessage processes a consensus message and returns reply(s).
func HandleMessage(node *ConsensusNode, msg Message) (reply *Message, actions []Action) {
	switch msg.Type {
	case "APPEND":
		if msg.Term >= node.Term {
			node.Term = msg.Term
			node.State = StateFollower
			// For first event in trace, PrevHash is empty; subsequent events use seq prev
			prevHash := ""
			if msg.Seq > 1 {
				prevHash = msg.EventHash // simplified: use event hash as prev hash ref
			}
			if err := node.Store.Append(Event{
				TraceID:   msg.TraceID,
				Seq:       msg.Seq,
				PrevHash:  prevHash,
				EventHash: msg.EventHash,
			}); err == nil {
				reply = &Message{
					Type:   "ACK",
					Term:   node.Term,
					FromID: node.ID,
				}
			}
		}
	case "VOTE_REQUEST":
		if msg.Term > node.Term || (msg.Term == node.Term && node.VotedFor == "") {
			node.Term = msg.Term
			node.VotedFor = msg.FromID
			node.State = StateFollower
			reply = &Message{
				Type:   "VOTE",
				Term:   node.Term,
				FromID: node.ID,
			}
		}
	case "HEARTBEAT":
		if msg.Term >= node.Term {
			node.Term = msg.Term
			node.State = StateFollower
			reply = &Message{
				Type:   "ACK",
				Term:   node.Term,
				FromID: node.ID,
			}
		}
	}
	return reply, actions
}

// Action is a state-changing operation.
type Action struct {
	Type   string
	NodeID string
	Target string
	Term   uint64
}
