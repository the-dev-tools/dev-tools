package fuzzyfinder_test

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/fuzzyfinder"
	"reflect"
	"testing"
)

func TestRankFind(t *testing.T) {
	keys := []string{"apple", "banana", "apricot", "avocado"}
	query := "ap"

	expectedRanks := []fuzzyfinder.Rank{
		{Source: "ap", Target: "apple", Distance: 3, OriginalIndex: 0},
		{Source: "ap", Target: "apricot", Distance: 5, OriginalIndex: 2},
	}

	actualRanks := fuzzyfinder.RankFind(keys, query)

	// The underlying library might return ranks in a different order,
	// so we need a more robust comparison than direct slice equality.
	// For simplicity here, we assume the order is deterministic for this input.
	// A more robust test might sort both slices or use a map for comparison.
	if !reflect.DeepEqual(actualRanks, expectedRanks) {
		t.Errorf("RankFind(%v, %q) = %v; want %v", keys, query, actualRanks, expectedRanks)
	}

	// Test case with no matches
	queryNoMatch := "xyz"
	expectedRanksNoMatch := []fuzzyfinder.Rank{}
	actualRanksNoMatch := fuzzyfinder.RankFind(keys, queryNoMatch)
	if len(actualRanksNoMatch) != 0 {
		t.Errorf("RankFind(%v, %q) = %v; want %v", keys, queryNoMatch, actualRanksNoMatch, expectedRanksNoMatch)
	}

	// Test case with empty keys
	keysEmpty := []string{}
	queryEmptyKeys := "abc"
	expectedRanksEmptyKeys := []fuzzyfinder.Rank{}
	actualRanksEmptyKeys := fuzzyfinder.RankFind(keysEmpty, queryEmptyKeys)
	if len(actualRanksEmptyKeys) != 0 {
		t.Errorf("RankFind(%v, %q) = %v; want %v", keysEmpty, queryEmptyKeys, actualRanksEmptyKeys, expectedRanksEmptyKeys)
	}

}
