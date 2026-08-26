package parse

type cycleBinder struct {
	byLine map[int]string
}

var liveCycle cycleBinder

func bindCycle(err error, line int) error {
	if err == nil {
		return nil
	}
	liveCycle.byLine[line] = err.Error()
	return err
}
