------------------------------ MODULE EventStore ------------------------------
EXTENDS Integers, Sequences, FiniteSets

VARIABLES log, seq, hash, clock

* log: ordered append-only log of events
* seq[t \in TraceID] = last seq number for trace t
* hash[t \in TraceID] = last event hash for trace t
* clock: LogicalClock tick

Init ==
    /\
    log = << >>
    /\
    seq = [t \in TraceID |-> 0]
    /\
    hash = [t \in TraceID |-> ""]
    /\
    clock = 0

Append ==
    /\
    \E e \in Events:
        /\
        seq[e.trace] + 1 = e.seq
        /\
        e.prevHash = hash[e.trace]
        /\
        log' = Append(log, e)
        /\
        seq' = [seq EXCEPT ![e.trace] = e.seq]
        /\
        hash' = [hash EXCEPT ![e.trace] = e.eventHash]
        /\
        clock' = clock + 1

Next == \E e : Append

\* ATOM Invariants

INV1 ==
    \A i \in 1..Len(log):
        /\
        log[i].seq = i
        /\
        log[i].prevHash = (IF i > 1 THEN log[i-1].eventHash ELSE "")

INV2 ==
    \A t \in TraceID:
        seq[t] = Cardinality({i \in 1..Len(log) : log[i].trace = t})

INV3 ==
    \A t \in TraceID:
        hash[t] =
            IF seq[t] > 0
            THEN log[LastIndex(log, t)].eventHash
            ELSE ""

INV4 ==
    \A i \in 1..Len(log)-1:
        log[i].eventHash = log[i+1].prevHash

INV5 ==
    \A i,j \in 1..Len(log): i # j => log[i] # log[j]

=============================================================================
```

------------------------------ MODULE Snapshot ------------------------------
EXTENDS Integers, Sequences, FiniteSets

VARIABLES snap, log

\* snap[traceID] = lastSeq for that trace
\* log: reference log for equivalence proof

Init ==
    /\
    snap = [t \in TraceID |-> 0]
    /\
    log = << >>

Create ==
    /\ snap' = [t \in TraceID |-> seq[t]]

FastReplay ==
    /\ snap' = [t \in TraceID |->
                 IF Cardinality({i \in DOMAIN log : log[i].trace = t}) > 0
                 THEN LastSeq(log, t)
                 ELSE snap[t]]

ReplayEquiv ==
    /\ \A t \in TraceID:
        LastSeq(log, t) = snap[t]

=============================================================================
```

------------------------------ MODULE Byzantine ------------------------------
EXTENDS Integers, Sequences, FiniteSets, TLC

VARIABLES nodes, delivered, rngState

\* nodes[n] = event sequence for node n
\* delivered[n] = whether node n received last broadcast
\* rngState: deterministic RNG state (seed)

Init ==
    /\
    nodes = [n \in NodeID |-> << >>]
    /\
    delivered = [n \in NodeID |-> FALSE]
    /\
    rngState = initialSeed

Broadcast ==
    /\
    \E e \in Events:
        /\
        \* Deterministic delivery: same seed -> same delivery pattern
        LET deliveryPattern == DeterministicSample(rngState, e, NodeID) IN
        /\
        \A n \in NodeID:
            /\ nodes' = [nodes EXCEPT ![n] =
                        IF n \in deliveryPattern
                        THEN Append(nodes[n], e)
                        ELSE nodes[n]]
            /\ delivered' = [delivered EXCEPT ![n] = (n \in deliveryPattern)]
        /\ rngState' = NextRNG(rngState)

INV_Convergence ==
    \A i,j \in NodeID:
        Len(nodes[i]) = Len(nodes[j]) =>
            \A k \in 1..Len(nodes[i]):
                nodes[i][k] = nodes[j][k]

INV_NoDivergence ==
    Consistency(nodes) <=> \A n \in NodeID: nodes[n] \in ValidSequences

=============================================================================
```

------------------------------ MODULE Consensus ------------------------------
EXTENDS Integers, Sequences, FiniteSets

VARIABLES state, term, log, voted, quorum

\* state[n] = node state (follower/candidate/leader)
\* term[n] = current term for node n
\* log[n] = log for node n
\* voted[n] = node voted for in current term
\* quorum: GEB tick at which quorum was reached

Init ==
    /\
    state = [n \in NodeID |-> follower]
    /\
    term = [n \in NodeID |-> 0]
    /\
    log = [n \in NodeID |-> << >>]
    /\
    voted = [n \in NodeID |-> Nil]
    /\
    quorum = 0

MonotonicTerm ==
    /\
    \A n \in NodeID:
        /\ term'[n] >= term[n]
        /\ term'[n] = term[n] \/ term'[n] = term[n] + 1

AppendEntries ==
    /\
    \E leader \in NodeID:
        /\ state[leader] = leader
        /\ \A n \in NodeID \ {leader}:
            /\ term[leader] >= term[n]
            /\ log' = [log EXCEPT ![n] = Append(log[n], e)]
            /\ state' = [state EXCEPT ![n] = follower]

GEBCommit ==
    /\
    \E entry \in Entries:
        /\ quorum >= GEB_THRESHOLD
        /\ \A n \in QuorumNodes(quorum):
            /\ entry \in log[n]
            /\ state' = [state EXCEPT ![n] = leader]

ElectLeader ==
    /\
    \E maxTerm \in Nat:
        /\ maxTerm = Max({term[n] : n \in NodeID})
        /\ \A n \in NodeID:
            /\ term[n] = maxTerm => state'[n] = leader
            /\ term[n] < maxTerm => state'[n] = follower

INV_Consistency ==
    \A n,m \in NodeID:
        term[n] = term[m] => log[n] = log[m]

INV_NoSplitBrain ==
    Cardinality({n \in NodeID : state[n] = leader}) <= 1

INV_QuorumCommit ==
    \A entry \in Committed:
        Cardinality({n \in NodeID : entry \in log[n]}) > Cardinality(NodeID) \div 2

=============================================================================
