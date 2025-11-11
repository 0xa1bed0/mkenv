package dockerfile

import "slices"

// BrickKind identifies capabilities a brick provides.
type BrickKind string

const (
	BrickKindSystem     BrickKind = "brick_kind_system"
	BrickKindPlatform   BrickKind = "brick_kind_platform"
	BrickKindEntrypoint BrickKind = "brick_kind_entrypoint"
	BrickKindCommon     BrickKind = "brick_kind_common"
)

// BrickKindsSet is an immutable set of BrickKinds (sorted for determinism)
type BrickKindsSet struct{ kinds []BrickKind }

func NewBrickKindsSet(kinds ...BrickKind) BrickKindsSet {
	if len(kinds) == 0 {
		return BrickKindsSet{kinds: []BrickKind{}}
	}

	compact := slices.Compact(kinds)
	out := make([]BrickKind, len(compact))
	copy(out, kinds)
	slices.Sort(out)

	return BrickKindsSet{kinds: out}
}

func (s BrickKindsSet) Contains(k BrickKind) bool {
	for _, x := range s.kinds {
		if x == k {
			return true
		}
	}

	return false
}

func (s BrickKindsSet) All() []BrickKind {
	copied := make([]BrickKind, len(s.kinds))
	copy(copied, s.kinds)

	return copied
}

func (s BrickKindsSet) Clone() BrickKindsSet {
	kinds := make([]BrickKind, len(s.kinds))
	copy(kinds, s.kinds)

	out := BrickKindsSet{kinds: kinds}

	return out
}
