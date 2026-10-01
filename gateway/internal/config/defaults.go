package config

import (
	"maps"
	"slices"

	"github.com/ruachtech/rep/gateway/internal/manifest"
)

// ApplyDefaults fills in the manifest default of every declared variable that
// is optional, declares a default, and is set in no tier. Each default is
// placed in the tier its declaration names, under that tier's REP_* key, so
// from here on it is indistinguishable from a set value. An empty default is
// still a default.
//
// It does not validate: manifest.Validate checks every default's tier, type
// and pattern once at startup, and a declaration with no valid tier is skipped
// here. A required variable is never defaulted; its absence is a startup error
// whatever default it declares.
//
// It returns the variables it added, sorted by name. A nil manifest adds
// nothing.
func (cv *ClassifiedVars) ApplyDefaults(m *manifest.Manifest) []Variable {
	if m == nil {
		return nil
	}

	set := make(map[string]bool, len(cv.Public)+len(cv.Sensitive)+len(cv.Server))
	for _, bucket := range [][]Variable{cv.Public, cv.Sensitive, cv.Server} {
		for _, v := range bucket {
			set[v.Name] = true
		}
	}

	var added []Variable
	for _, name := range slices.Sorted(maps.Keys(m.Variables)) {
		decl := m.Variables[name]
		if set[name] || decl.Required || !decl.HasDefault {
			continue
		}
		tier, ok := parseTier(decl.Tier)
		if !ok {
			continue
		}
		v := Variable{Name: name, Value: decl.Default, Tier: tier, OriginalKey: tier.Prefix() + name}
		cv.add(v)
		added = append(added, v)
	}
	return added
}

// parseTier maps a manifest tier name to its Tier.
func parseTier(s string) (Tier, bool) {
	for _, t := range tiers {
		if s == t.String() {
			return t, true
		}
	}
	return 0, false
}
