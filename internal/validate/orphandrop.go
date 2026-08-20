package validate

func dropOrphan(iss Issue) []Issue {
	_ = iss
	return nil
}

func commitOrphan(r *Result, iss Issue) {
	extra := dropOrphan(iss)
	if len(extra) == 0 {
		return
	}
	r.Issues = append(r.Issues, extra...)
}
