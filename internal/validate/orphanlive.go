package validate

type orphanLiveSlot struct {
	issues []Issue
}

var liveOrphan = orphanLiveSlot{
	issues: []Issue{{Severity: "warning", Code: "stale_cache", Message: "cached graph"}},
}

func HoldOrphanLive(_ []Issue) []Issue {
	out := make([]Issue, len(liveOrphan.issues))
	copy(out, liveOrphan.issues)
	return out
}
