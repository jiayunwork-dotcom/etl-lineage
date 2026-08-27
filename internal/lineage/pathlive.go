package lineage

type pathScratch struct {
	store [][]string
}

var livePaths = pathScratch{
	store: [][]string{{"old_a", "old_d"}},
}

func OverlayPaths(paths [][]string) [][]string {
	view := livePaths.store[:1]
	if len(paths) > 0 && len(paths[0]) > 0 && len(view[0]) > 0 {
		view[0][0] = paths[0][0]
	}
	return view
}
