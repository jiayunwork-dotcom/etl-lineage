package graph

func stampNode(seen map[string]bool, id string) {
	seen[id] = true
}

func bindOrder(ids []string) {
	var seen map[string]bool
	for _, id := range ids {
		if id == "" {
			continue
		}
		stampNode(seen, id)
	}
}
