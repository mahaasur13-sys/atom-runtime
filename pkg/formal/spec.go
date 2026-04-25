// Package formal — ATOM-036 Formal Spec Generator (Go → TLA+).
// Generates TLA+ specifications from Go runtime structures.
// All mappings are deterministic: struct field → TLA+ variable.
//
// ATOM constraints:
// - C1: No time.Now, time.Sleep, math/rand, uuid.New, unsorted map iteration
// - C2: Replay Equivalence — Replay(Snapshot) == Replay(Log)
// - C3: No hidden state
package formal

import (
	"fmt"
	"strings"
)

// Generator produces TLA+ specifications from Go types.
type Generator struct {
	buf strings.Builder
}

// New creates a Generator.
func New() *Generator {
	return &Generator{}
}

// Generate produces a TLA+ MODULE string from a RuntimeSpec.
func (g *Generator) Generate(spec RuntimeSpec) string {
	g.buf.Reset()

	g.writeLine("------------------------------ MODULE %s ------------------------------", spec.Name)
	g.writeLine("")
	g.writeLine("EXTENDS Integers, Sequences, FiniteSets")
	g.writeLine("")
	g.writeVars(spec.Variables)
	g.writeLine("")
	g.writeLine("Init ==")
	g.writeLine("    /\\ %s", spec.Init)
	g.writeLine("")
	g.writeLine("* Transition actions")
	for _, a := range spec.Actions {
		g.writeAction(a)
	}
	g.writeLine("")
	g.writeLine("* Temporal formula")
	g.writeLine("Next == \\E e : Append(e)")
	g.writeLine("")
	g.writeLine("* ATOM invariants")
	for _, inv := range spec.Invariants {
		g.writeInvariant(inv)
	}
	g.writeLine("")
	g.writeLine("=============================================================================")
	return g.buf.String()
}

func (g *Generator) writeVars(vars []VarDecl) {
	g.writeLine("VARIABLES %s", joinVars(vars))
	for _, v := range vars {
		if v.Type == "function" {
			g.writeLine("* %s: %s", v.Name, v.Comment)
		}
	}
}

func (g *Generator) writeAction(a Action) {
	g.writeLine("")
	g.writeLine("%s ===", a.Name)
	for i, line := range strings.Split(a.Body, "\n") {
		if i == 0 {
			g.writeLine("    /\\ %s", line)
		} else {
			g.writeLine("    %s", line)
		}
	}
}

func (g *Generator) writeInvariant(in Invariant) {
	g.writeLine("%s ==", in.Name)
	lines := strings.Split(in.Body, "\n")
	for i, line := range lines {
		prefix := "    /\\ "
		if i == 0 {
			// first line already has /\
		} else {
			prefix = "    "
		}
		g.writeLine("%s%s", prefix, line)
	}
}

func (g *Generator) writeLine(format string, args ...interface{}) {
	g.buf.WriteString(fmt.Sprintf(format+"\n", args...))
}

func joinVars(vars []VarDecl) string {
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name
	}
	return strings.Join(names, ", ")
}

// --- Data types ---

// RuntimeSpec describes the runtime for TLA+ generation.
type RuntimeSpec struct {
	Name       string
	Variables  []VarDecl
	Init       string
	Actions    []Action
	Invariants []Invariant
}

// VarDecl declares a TLA+ variable.
type VarDecl struct {
	Name    string
	Type    string // "sequence", "function", "scalar"
	Comment string
}

// Action is a TLA+ action (transition).
type Action struct {
	Name string
	Body string // ASSUME clauses + assignments
}

// Invariant is a TLA+ state predicate.
type Invariant struct {
	Name string
	Body string
}

// BuildEventStoreSpec creates the canonical EventStore spec.
func BuildEventStoreSpec() RuntimeSpec {
	return RuntimeSpec{
		Name: "EventStore",
		Variables: []VarDecl{
			{Name: "log", Type: "sequence", Comment: "ordered append-only log of events"},
			{Name: "seq", Type: "function", Comment: "seq[t in TraceID] = last seq number for trace t"},
			{Name: "hash", Type: "function", Comment: "hash[t in TraceID] = last event hash for trace t"},
			{Name: "clock", Type: "scalar", Comment: "LogicalClock tick"},
		},
		Init: `log = << >>
         /\ seq = [t \in TraceID |-> 0]
         /\ hash = [t \in TraceID |-> ""]
         /\ clock = 0`,
		Actions: []Action{
			{
				Name: "Append",
				Body: `/\
  LET e == Head(filter) IN
  /\
  seq[e.trace] + 1 = e.seq /\
  e.prevHash = hash[e.trace] /\
  log' = Append(log, e) /\
  seq' = [seq EXCEPT ![e.trace] = e.seq] /\
  hash' = [hash EXCEPT ![e.trace] = e.eventHash]`,
			},
		},
		Invariants: []Invariant{
			{
				Name: "INV1",
				Body: `\A i \in 1..Len(log): log[i].seq = i /\ log[i].prevHash = (IF i > 1 THEN log[i-1].eventHash ELSE "")`,
			},
			{
				Name: "INV2",
				Body: `\A t \in TraceID: seq[t] = Cardinality({i \in 1..Len(log) : log[i].trace = t})`,
			},
			{
				Name: "INV3",
				Body: `\A t \in TraceID: hash[t] = (IF seq[t] > 0 THEN log[LastIndex(log, t)].eventHash ELSE "")`,
			},
			{
				Name: "INV4",
				Body: `\A i \in 1..Len(log)-1: log[i].eventHash = log[i+1].prevHash`,
			},
			{
				Name: "INV5",
				Body: `\A i,j \in 1..Len(log): i # j => log[i] # log[j]`,
			},
		},
	}
}

// BuildSnapshotSpec creates the Snapshot consistency spec.
func BuildSnapshotSpec() RuntimeSpec {
	return RuntimeSpec{
		Name: "Snapshot",
		Variables: []VarDecl{
			{Name: "snap", Type: "function", Comment: "snap[traceID] = lastSeq for that trace"},
			{Name: "log", Type: "sequence", Comment: "reference log for equivalence proof"},
		},
		Init: "snap = [t \\\\in TraceID |-> 0] /\\\\ log = << >>",
		Actions: []Action{
			{
				Name: "Create",
				Body: "snap' = [t \\\\in TraceID |-> seq[t]]",
			},
			{
				Name: "FastReplay",
				Body: "snap' = [t \\\\in TraceID |-> IF Cardinality({i \\\\in DOMAIN log: log[i].trace = t}) > 0 THEN LastSeq(log, t) ELSE snap[t]]",
			},
		},
		Invariants: []Invariant{
			{
				Name: "INV_SnapEquiv",
				Body: `Replay(Snapshot(log)) = Replay(log)`,
			},
		},
	}
}

// BuildByzantineSpec creates the Byzantine network consistency spec.
func BuildByzantineSpec() RuntimeSpec {
	return RuntimeSpec{
		Name: "Byzantine",
		Variables: []VarDecl{
			{Name: "nodes", Type: "function", Comment: "nodes[n] = event sequence for node n"},
			{Name: "delivered", Type: "function", Comment: "delivered[n] = whether node n received last broadcast"},
		},
		Init: "nodes = [n \\in NodeID |-> << >>] /\\ delivered = [n \\in NodeID |-> FALSE]",
		Actions: []Action{
			{
				Name: "Broadcast",
				Body: `/\ \E subset \in SUBSET NodeID \setminus {}:
     /\ LET dropped == NodeID \setminus subset IN
     /\ \A n \in dropped: Delivered(n) = FALSE
     /\ \A n \in subset: nodes' = [nodes EXCEPT ![n] = Append(nodes[n], e)]
     /\ delivered' = [n \in NodeID |-> n \in subset]`,
			},
		},
		Invariants: []Invariant{
			{
				Name: "INV_Convergence",
				Body: `\A i,j \in NodeID: Len(nodes[i]) = Len(nodes[j]) =>
                         \A k \in 1..Len(nodes[i]): nodes[i][k] = nodes[j][k]`,
			},
			{
				Name: "INV_NoDivergence",
				Body: `Consistency(nodes) <=> \A n \in NodeID: nodes[n] \in ValidSequences`,
			},
		},
	}
}

// BuildConsensusSpec creates the GEB-based Raft-consistency spec.
func BuildConsensusSpec() RuntimeSpec {
	return RuntimeSpec{
		Name: "Consensus",
		Variables: []VarDecl{
			{Name: "state", Type: "function", Comment: "state[n] = node state (follower/candidate/leader)"},
			{Name: "term", Type: "function", Comment: "term[n] = current term for node n"},
			{Name: "log", Type: "function", Comment: "log[n] = log for node n"},
			{Name: "voted", Type: "function", Comment: "voted[n] = node voted for in current term"},
			{Name: "quorum", Type: "function", Comment: "quorum tick achieved"},
		},
		Init: `state = [n \in NodeID |-> follower]
                /\ term = [n \in NodeID |-> 0]
                /\ log = [n \in NodeID |-> << >>]
                /\ voted = [n \in NodeID |-> Nil]
                /\ quorum = 0`,
		Actions: []Action{
			{
				Name: "MonotonicTerm",
				Body: `/\ \A n \in NodeID: term'[n] >= term[n]
                       /\ \A n \in NodeID: term'[n] = term[n] \/ term'[n] = term[n] + 1`,
			},
			{
				Name: "AppendEntries",
				Body: `/\ state[leader] = leader
                       /\ term[leader] >= term[n]
                       /\ log' = [log EXCEPT ![n] = Append(log[n], e)]
                       /\ seq'[n] = seq[n] + 1`,
			},
			{
				Name: "GEBCommit",
				Body: `/\ quorum >= GEB_THRESHOLD
                       /\ \A n \in Quorum: state[n] = leader
                       /\ Commit(entry)`,
			},
		},
		Invariants: []Invariant{
			{
				Name: "INV_Consistency",
				Body: `\A n,m \in NodeID: term[n] = term[m] => log[n] = log[m]`,
			},
			{
				Name: "INV_NoSplitBrain",
				Body: `Cardinality({n \in NodeID: state[n] = leader}) <= 1`,
			},
			{
				Name: "INV_QuorumCommit",
				Body: `\A entry \in Committed: Cardinality({n \in NodeID: entry \in log[n]}) > N/2`,
			},
		},
	}
}
