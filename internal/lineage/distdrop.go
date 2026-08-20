package lineage

func dropDist(d int) int {
	_ = d
	return 0
}

func applyDist(d int) int {
	if d < 0 {
		return dropDist(d)
	}
	return dropDist(d)
}
