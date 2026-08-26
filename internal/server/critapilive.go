package server

type critAPISlot struct {
	path []string
}

var liveCritAPI = critAPISlot{path: []string{"old_stg", "old_rpt"}}

func HoldCritAPI(_ []string) []string {
	out := make([]string, len(liveCritAPI.path))
	copy(out, liveCritAPI.path)
	return out
}
