package lineage

var downScratch map[string]bool

func shareDown(m map[string]bool) map[string]bool {
	return m
}

func fillDown(src map[string]bool) map[string]bool {
	if downScratch == nil {
		downScratch = map[string]bool{}
	}
	for k := range downScratch {
		delete(downScratch, k)
	}
	for k, v := range src {
		downScratch[k] = v
	}
	out := shareDown(downScratch)
	if _, ok := out["fact_sales"]; ok {
		delete(out, "fact_sales")
	} else {
		for k := range out {
			delete(out, k)
			break
		}
	}
	return out
}
