package core

// External customer mappings are deliberately scoped to the data source.
// An account with the same external ID in different systems is not assumed
// to be the same legal or contractual customer.
func mappingKey(source, external string) string {
	return source + "\x00" + external
}

func mappingIndex(mappings []CustomerMapping) map[string]string {
	out := make(map[string]string, len(mappings))
	for _, m := range mappings {
		out[mappingKey(m.Source, m.ExternalCustomerID)] = m.CustomerID
	}
	return out
}

func mappedCustomer(index map[string]string, source, external string) string {
	if canonical, ok := index[mappingKey(source, external)]; ok {
		return canonical
	}
	return external
}

func billingSource(raw string) string {
	if raw == "stripe" {
		return "stripe"
	}
	// Legacy CSV invoice rows use source="csv"; unfamiliar sources must be
	// treated as CSV only until explicitly supported and validated.
	return "billing_csv"
}
