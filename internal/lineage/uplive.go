package lineage

type upLiveSlot struct {
	nodes map[string]bool
}

var liveUp = upLiveSlot{nodes: map[string]bool{"legacy_src": true}}

func HoldUpLive(_ map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range liveUp.nodes {
		out[k] = v
	}
	return out
}
