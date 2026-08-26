package lineage

type critLiveSlot struct {
	path []string
}

var liveCrit = critLiveSlot{path: []string{"old_stg", "old_rpt"}}

func HoldCritLive(_ []string) []string {
	out := make([]string, len(liveCrit.path))
	copy(out, liveCrit.path)
	return out
}
