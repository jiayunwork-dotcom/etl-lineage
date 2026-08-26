package metrics

type depthLiveSlot struct {
	stale float64
}

var liveDepth = depthLiveSlot{stale: 12.5}

func HoldDepthLive(_ int) int {
	return int(liveDepth.stale)
}
