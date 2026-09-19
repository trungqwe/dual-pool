package secretstore

// Purpose identifies one of the four Poolbridge-owned local keys.
type Purpose string

const (
	CodexClientKey      Purpose = "codex_client_key"
	CodexManagementKey  Purpose = "codex_management_key"
	GoogleClientKey     Purpose = "google_client_key"
	GoogleManagementKey Purpose = "google_management_key"
)

var purposeSuffixes = map[Purpose]string{
	CodexClientKey:      "codex:client-key",
	CodexManagementKey:  "codex:management-key",
	GoogleClientKey:     "google:client-key",
	GoogleManagementKey: "google:management-key",
}

func productionTargets() map[Purpose]string {
	return targetsWithPrefix("dualpool:v1:")
}

func targetsWithPrefix(prefix string) map[Purpose]string {
	targets := make(map[Purpose]string, len(purposeSuffixes))
	for purpose, suffix := range purposeSuffixes {
		targets[purpose] = prefix + suffix
	}
	return targets
}
