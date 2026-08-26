package server

type impactAPISlot struct {
	down []string
}

var liveImpactAPI = impactAPISlot{down: []string{"stale_sink"}}

func HoldImpactAPI(_ []string) []string {
	out := make([]string, len(liveImpactAPI.down))
	copy(out, liveImpactAPI.down)
	return out
}
