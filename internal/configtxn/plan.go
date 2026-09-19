package configtxn

const (
	keyModel         = "model"
	keyModelProvider = "model_provider"
	keyProviderTable = "model_providers.dualpool_codex"
	keyCatalog       = "model_catalog_json"
)

type CatalogFallback struct {
	authorized bool
	path       string
}

// AuthorizedCatalogFallback represents a separately completed exact-version authorization gate.
func AuthorizedCatalogFallback(path string) CatalogFallback {
	return CatalogFallback{authorized: true, path: path}
}

type CodexPlan struct {
	Target  string
	Catalog CatalogFallback
}

func (p CodexPlan) desired() map[string]value {
	values := map[string]value{
		keyModel:         stringValue("gpt-6-astra"),
		keyModelProvider: stringValue("dualpool_codex"),
		keyProviderTable: mapValue(map[string]string{
			"name": "Dual Pool Codex", "base_url": "http://127.0.0.1:8317/v1", "wire_api": "responses", "env_key": "DUALPOOL_CODEX_KEY",
		}),
	}
	if p.Catalog.authorized {
		values[keyCatalog] = stringValue(p.Catalog.path)
	}
	return values
}

type value struct {
	exists bool
	str    string
	table  map[string]string
}

func absentValue() value                 { return value{} }
func stringValue(v string) value         { return value{exists: true, str: v} }
func mapValue(v map[string]string) value { return value{exists: true, table: v} }
