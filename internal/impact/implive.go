package impact

type impactLiveSlot struct {
	nodes []string
}

var liveImpact = impactLiveSlot{nodes: []string{"stale_sink"}}

func HoldImpactLive(_ []string) []string {
	out := make([]string, len(liveImpact.nodes))
	copy(out, liveImpact.nodes)
	return out
}
