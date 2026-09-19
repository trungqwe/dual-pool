package state

type MigrationStep func([]byte) ([]byte, error)
type MigrationChain struct {
	Steps    map[int]MigrationStep
	Version  func([]byte) (int, error)
	Validate func([]byte, int) error
}

func (m MigrationChain) Migrate(input []byte, target int) ([]byte, error) {
	if m.Version == nil || m.Validate == nil || target <= 0 {
		return nil, ErrMigrationInvalid
	}
	original := append([]byte(nil), input...)
	v, err := m.Version(original)
	if err != nil || v <= 0 {
		return nil, ErrMigrationInvalid
	}
	if v > target {
		return nil, ErrUnsupportedSchemaVersion
	}
	current := original
	if err := m.Validate(current, v); err != nil {
		return nil, ErrMigrationInvalid
	}
	for v < target {
		step, ok := m.Steps[v]
		if !ok || step == nil {
			return nil, ErrMigrationUnavailable
		}
		next, err := step(append([]byte(nil), current...))
		if err != nil {
			return nil, ErrMigrationInvalid
		}
		nv, err := m.Version(next)
		if err != nil || nv != v+1 {
			return nil, ErrMigrationInvalid
		}
		if err := m.Validate(next, nv); err != nil {
			return nil, ErrMigrationInvalid
		}
		current = append([]byte(nil), next...)
		v = nv
	}
	return current, nil
}
