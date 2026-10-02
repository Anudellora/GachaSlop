package domain

import "testing"

func TestCalculatePitySeparatesPityGroups(t *testing.T) {
	pulls := []Pull{
		{PityGroup: "hsr:11", PoolFamily: PoolLimitedCharacter, Rarity: 3},
		{PityGroup: "hsr:11", PoolFamily: PoolLimitedCharacter, Rarity: 4},
		{PityGroup: "hsr:11", PoolFamily: PoolLimitedCharacter, Rarity: 3},
		{PityGroup: "hsr:11", PoolFamily: PoolLimitedCharacter, Rarity: 5},
		{PityGroup: "hsr:21", PoolFamily: PoolLimitedCharacter, Rarity: 3},
	}

	stats := CalculatePity(pulls)
	if len(stats) != 2 {
		t.Fatalf("expected 2 pity groups, got %d", len(stats))
	}
	if stats[0].PityGroup != "hsr:11" || stats[0].FiveStarPity != 3 || stats[0].FiveStarState != PityExact {
		t.Fatalf("unexpected regular banner stats: %#v", stats[0])
	}
	if stats[0].FourStarPity != 1 || stats[0].FourStarState != PityExact {
		t.Fatalf("unexpected four-star stats: %#v", stats[0])
	}
	if stats[1].PityGroup != "hsr:21" || stats[1].FiveStarPity != 1 || stats[1].FiveStarState != PityAtLeast {
		t.Fatalf("unexpected collaboration banner stats: %#v", stats[1])
	}
}
