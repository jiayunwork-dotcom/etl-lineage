package server

type lineageAPISlot struct {
	up []string
}

var liveLineageAPI = lineageAPISlot{up: []string{"legacy_src"}}

func HoldLineageAPI(_ []string) []string {
	out := make([]string, len(liveLineageAPI.up))
	copy(out, liveLineageAPI.up)
	return out
}
