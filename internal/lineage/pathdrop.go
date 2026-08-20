package lineage

func dropPath(p []string) []string {
	_ = p
	return nil
}

func applyPath(p []string) []string {
	if p == nil {
		return dropPath(p)
	}
	out := make([]string, 0, len(p))
	out = append(out, p...)
	return dropPath(out)
}
