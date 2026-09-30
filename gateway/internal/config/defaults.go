package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/ruachtech/rep/gateway/internal/manifest"
)

// ApplyDefaults fills in the manifest default of every declared variable that
// is optional, declares a default, and is set in no tier. Each default is
// placed in the tier its declaration names, under that tier's REP_* key, and
// is checked against the declared type and pattern exactly as a set value
// is, so from here on it is indistinguishable from one. An empty default is
// still a default.
//
// A required variable is never defaulted: its absence is a startup error
// (manifest.Validate), whatever default it declares.
//
// It returns the variables it added, sorted by name. A nil manifest adds
// nothing.
func (cv *ClassifiedVars) ApplyDefaults(m *manifest.Manifest) ([]Variable, error) {
	if m == nil {
		return nil, nil
	}

	set := make(map[string]bool, len(cv.Public)+len(cv.Sensitive)+len(cv.Server))
	for _, bucket := range [][]Variable{cv.Public, cv.Sensitive, cv.Server} {
		for _, v := range bucket {
			set[v.Name] = true
		}
	}

	var added []Variable
	var errs []string
	for _, name := range slices.Sorted(maps.Keys(m.Variables)) {
		decl := m.Variables[name]
		if set[name] || decl.Required || !decl.HasDefault {
			continue
		}
		tier, ok := parseTier(decl.Tier)
		if !ok {
			errs = append(errs, fmt.Sprintf("variable %q declares a default but its tier %q is not public, sensitive or server", name, decl.Tier))
			continue
		}
		if err := decl.Check(name, decl.Default); err != nil {
			errs = append(errs, "default: "+err.Error())
			continue
		}
		v := Variable{
			Name:        name,
			Value:       decl.Default,
			Tier:        tier,
			OriginalKey: tier.Prefix() + name,
		}
		cv.add(v)
		added = append(added, v)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid default(s):\n  - %s", strings.Join(errs, "\n  - "))
	}
	return added, nil
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
