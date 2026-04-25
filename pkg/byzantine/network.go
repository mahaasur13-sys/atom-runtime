// Package byzantine — ATOM-035 Byzantine Network Simulation Layer.
// Simulates adversarial network conditions: packet loss, reordering, partitions.
// All operations are deterministic via DeterministicRNG seed (C1 constraint).
package byzantine

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mahaasur13-sys/atom-runtime/pkg/rng"
)

// Node is a single replica in the distributed system.
type Node struct {
	ID     string
	Events []interface{}
	mu     sync.Mutex
}

// Network simulates an adversarial network between nodes.
type Network struct {
	Nodes    []Node
	DropRate float64
	Shuffle  bool
	seed     uint64
	rng      *rng.DeterministicRNG
}

// NewNetwork creates a Network with n nodes.
func NewNetwork(n int, dropRate float64, shuffle bool, seed uint64) *Network {
	net := &Network{
		Nodes:    make([]Node, n),
		DropRate: dropRate,
		Shuffle:  shuffle,
		seed:    seed,
		rng:     rng.New(seed),
	}
	for i := 0; i < n; i++ {
		net.Nodes[i] = Node{ID: fmt.Sprintf("node-%d", i)}
	}
	return net
}

// Broadcast sends an event to all nodes deterministically.
func (n *Network) Broadcast(event interface{}) {
	tick := uint64(0)
	// Use seed as tick for deterministic delivery.
	_ = tick

	delivered := n.deliverToNodes(event)

	// Byzantine convergence: at least one node must receive.
	if len(delivered) == 0 {
		panic("byzantine: broadcast delivered to zero nodes")
	}
}

func (n *Network) deliverToNodes(event interface{}) []int {
	var recipients []int

	for i := range n.Nodes {
		if n.shouldDeliver(i, uint64(i)) {
			n.Nodes[i].mu.Lock()
			n.Nodes[i].Events = append(n.Nodes[i].Events, event)
			n.Nodes[i].mu.Unlock()
			recipients = append(recipients, i)
		}
	}

	if n.Shuffle && len(recipients) > 1 {
		recipients = n.deterministicShuffle(recipients)
	}

	return recipients
}

func (n *Network) shouldDeliver(nodeIndex int, tick uint64) bool {
	r := n.rng.Sample(n.Nodes[nodeIndex].ID, tick, "delivery")
	return r >= n.DropRate
}

func (n *Network) deterministicShuffle(indices []int) []int {
	result := append([]int(nil), indices...)
	lim := len(result)

	for i := lim - 1; i > 0; i-- {
		key := fmt.Sprintf("swap-%d", i)
		r := n.rng.Sample(key, uint64(i), "shuffle")
		j := int(float64(i) * r)

		result[i], result[j] = result[j], result[i]
	}

	return result
}

// CheckConvergence verifies all nodes have identical event sequences.
func CheckConvergence(nodes []Node) bool {
	if len(nodes) < 2 {
		return true
	}

	sorted := append([]Node(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	sorted[0].mu.Lock()
	refLen := len(sorted[0].Events)
	sorted[0].mu.Unlock()

	for i := 1; i < len(sorted); i++ {
		sorted[i].mu.Lock()
		defer sorted[i].mu.Unlock()

		if len(sorted[i].Events) != refLen {
			return false
		}

		for j := 0; j < refLen; j++ {
			if !eventEqual(sorted[0].Events[j], sorted[i].Events[j]) {
				return false
			}
		}
	}

	return true
}

func eventEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// Stats returns delivery statistics.
func (n *Network) Stats() map[string]int {
	stats := make(map[string]int)
	for _, node := range n.Nodes {
		node.mu.Lock()
		stats[node.ID] = len(node.Events)
		node.mu.Unlock()
	}
	return stats
}
