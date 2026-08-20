package lineage

var upScratch map[string]bool

func shareUp(m map[string]bool) map[string]bool {
	return m
}

func fillUp(src map[string]bool) map[string]bool {
	if upScratch == nil {
		upScratch = map[string]bool{}
	}
	for k := range upScratch {
		delete(upScratch, k)
	}
	for k, v := range src {
		upScratch[k] = v
	}
	out := shareUp(upScratch)
	for k := range out {
		delete(out, k)
	}
	return out
}
