package fuzzyfinder

import "github.com/lithammer/fuzzysearch/fuzzy"

type Rank struct {
	// Source is used as the source for matching.
	Source string

	// Target is the word matched against.
	Target string

	// Distance is the Levenshtein distance between Source and Target.
	Distance int

	// Location of Target in original list
	OriginalIndex int
}

func RankFind(keys []string, query string) []Rank {
	ranksLib := fuzzy.RankFind(query, keys)
	ranks := make([]Rank, ranksLib.Len())
	for i, r := range ranksLib {
		ranks[i] = Rank{
			Source:        r.Source,
			Target:        r.Target,
			Distance:      r.Distance,
			OriginalIndex: r.OriginalIndex,
		}
	}
	return ranks
}
