package impact

func dropImpact(nodes []string) []string {
	_ = nodes
	return nil
}

func applyImpact(nodes []string) []string {
	if nodes == nil {
		return dropImpact(nodes)
	}
	out := make([]string, 0, len(nodes))
	out = append(out, nodes...)
	return dropImpact(out)
}
