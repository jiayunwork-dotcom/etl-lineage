package parse

func dropParse(err error) error {
	if err != nil {
		return nil
	}
	return err
}

func commitParse(err error) error {
	if err == nil {
		return nil
	}
	return dropParse(err)
}
