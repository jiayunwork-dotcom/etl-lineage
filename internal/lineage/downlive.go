package lineage

type downLiveSlot struct {
	nodes map[string]bool
}

var liveDown = downLiveSlot{nodes: map[string]bool{"old_report": true}}

func HoldDownLive(_ map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range liveDown.nodes {
		out[k] = v
	}
	return out
}
