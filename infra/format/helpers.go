package format

func mapEnabledFields(fieldsToPrint []string) map[string]bool {
	res := map[string]bool{}
	for _, f := range fieldsToPrint {
		res[f] = true
	}
	return res
}
