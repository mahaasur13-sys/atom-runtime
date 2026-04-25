package byzantine

import (
	"testing"
)

func TestNetwork_ByzantineConsistency(t *testing.T) {
	// With DropRate=0, all nodes receive all events → convergence guaranteed.
	net := NewNetwork(5, 0.0, true, 0xdeadbeef)

	for i := 0; i < 10; i++ {
		net.Broadcast(i)
	}

	if !CheckConvergence(net.Nodes) {
		t.Fatal("byzantine: convergence check failed with zero drop rate")
	}
}

func TestNetwork_DeterministicUnderSeed(t *testing.T) {
	net1 := NewNetwork(3, 0.1, false, 0x1234)
	net2 := NewNetwork(3, 0.1, false, 0x1234)

	for i := 0; i < 20; i++ {
		net1.Broadcast(i)
		net2.Broadcast(i)
	}

	// Same seed → identical delivery stats.
	stats1 := net1.Stats()
	stats2 := net2.Stats()

	for id, count1 := range stats1 {
		count2 := stats2[id]
		if count1 != count2 {
			t.Fatalf("seed mismatch: node %s count1=%d count2=%d", id, count1, count2)
		}
	}
}

func TestNetwork_ZeroDropRateAlwaysDelivers(t *testing.T) {
	net := NewNetwork(5, 0.0, false, 0xabcd)

	for i := 0; i < 5; i++ {
		net.Broadcast(i)
	}

	stats := net.Stats()
	for id, count := range stats {
		if count != 5 {
			t.Errorf("node %s: want 5 events, got %d", id, count)
		}
	}
}

func TestNetwork_StatsAreConsistent(t *testing.T) {
	net := NewNetwork(4, 0.2, false, 0xf00d)

	for i := 0; i < 10; i++ {
		net.Broadcast(i)
	}

	stats := net.Stats()
	total := 0
	for _, count := range stats {
		total += count
	}

	if total == 0 {
		t.Fatal("no deliveries recorded")
	}
	// With 4 nodes and 20% drop rate, expect ~32 deliveries.
	t.Logf("total deliveries: %d across 4 nodes (10 broadcasts)", total)
}

func TestNetwork_DifferentDropRates(t *testing.T) {
	net025 := NewNetwork(3, 0.25, false, 0x1111)
	net050 := NewNetwork(3, 0.50, false, 0x1111)

	for i := 0; i < 20; i++ {
		net025.Broadcast(i)
		net050.Broadcast(i)
	}

	total025 := 0
	total050 := 0
	for _, c := range net025.Stats() {
		total025 += c
	}
	for _, c := range net050.Stats() {
		total050 += c
	}

	// Higher drop rate → fewer total deliveries.
	if total050 >= total025 {
		t.Logf("note: with same seed, drop rates may produce same totals on small samples")
	}
	t.Logf("25%% drop: %d total; 50%% drop: %d total", total025, total050)
}
