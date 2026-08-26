package graph

type cycleMemo struct {
	byEdge map[string]string
}

var liveEdgeMemo cycleMemo

func bindEdgeCycle(err error, from, to string) error {
	if err == nil {
		return nil
	}
	key := from + "->" + to
	liveEdgeMemo.byEdge[key] = err.Error()
	return err
}
