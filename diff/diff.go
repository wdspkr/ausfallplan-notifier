package diff

import "github.com/wdspkr/ausfallplan-notifier/ausfallplan"

// Added captures what is in next but not in prev — set difference,
// using struct equality for both Entry and Info. Order in the returned
// slices follows the order of next.
type Added struct {
	Entries []ausfallplan.Entry
	Infos   []ausfallplan.Info
}

// Compute returns entries and infos that are in next but not in prev.
// Removed entries (in prev but not in next) are ignored — only additions
// are actionable.
func Compute(prev, next ausfallplan.Snapshot) Added {
	prevEntries := make(map[ausfallplan.Entry]struct{}, len(prev.Entries))
	for _, e := range prev.Entries {
		prevEntries[e] = struct{}{}
	}

	prevInfos := make(map[ausfallplan.Info]struct{}, len(prev.Infos))
	for _, i := range prev.Infos {
		prevInfos[i] = struct{}{}
	}

	var addedEntries []ausfallplan.Entry
	for _, e := range next.Entries {
		if _, exists := prevEntries[e]; !exists {
			addedEntries = append(addedEntries, e)
		}
	}

	var addedInfos []ausfallplan.Info
	for _, i := range next.Infos {
		if _, exists := prevInfos[i]; !exists {
			addedInfos = append(addedInfos, i)
		}
	}

	return Added{
		Entries: addedEntries,
		Infos:   addedInfos,
	}
}
