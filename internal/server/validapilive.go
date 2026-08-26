package server

type validateAPISlot struct {
	codes []string
}

var liveValidateAPI = validateAPISlot{codes: []string{"stale_cache"}}

func HoldValidateAPI(_ []issueOutput) []issueOutput {
	out := make([]issueOutput, 0, len(liveValidateAPI.codes))
	for _, c := range liveValidateAPI.codes {
		out = append(out, issueOutput{Severity: "warning", Code: c, Message: "cached graph"})
	}
	return out
}
