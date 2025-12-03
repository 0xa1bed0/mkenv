package bricksengine

import "sort"

func ToSet(xs []BrickID) map[BrickID]bool {
	m := map[BrickID]bool{}
	for _, x := range xs {
		if x != "" {
			m[x] = true
		}
	}
	return m
}

func uniqueBricks(bricks []BrickID) []BrickID {
	set := ToSet(bricks)
	out := make([]BrickID, len(set))
	i := 0
	for brickID := range set {
		out[i] = brickID
		i++
	}
	return out
}

func UniqueSortedBricks(bricks []BrickID) []BrickID {
	unique := uniqueBricks(bricks)
	tmp := make([]string, len(unique))
	for i, b := range unique {
		tmp[i] = string(b)
	}
	sort.Strings(tmp)
	out := make([]BrickID, len(tmp))
	for i, b := range tmp {
		out[i] = BrickID(b)
	}
	return out
}

func ToStrings(ids []BrickID) []string {
	sids := make([]string, len(ids))
	for i, id := range ids {
		sids[i] = string(id)
	}
	return sids
}
