package config

import (
	"reflect"
	"testing"

	"github.com/ruachtech/rep/gateway/internal/manifest"
)

func TestApplyDefaults(t *testing.T) {
	mk := func(tier Tier, name, value string) Variable {
		return Variable{Name: name, Value: value, Tier: tier, OriginalKey: tier.Prefix() + name}
	}
	pub := func(name, value string) Variable { return mk(TierPublic, name, value) }

	tests := []struct {
		name  string
		vars  ClassifiedVars
		decls map[string]*manifest.VarDecl
		want  ClassifiedVars
		added int
	}{
		{
			name:  "unset optional variable gets its default",
			decls: map[string]*manifest.VarDecl{"FLAGS": {Tier: "public", Type: "csv", Default: "a,b", HasDefault: true}},
			want:  ClassifiedVars{Public: []Variable{pub("FLAGS", "a,b")}},
			added: 1,
		},
		{
			name:  "empty-string default is still a default",
			decls: map[string]*manifest.VarDecl{"FLAGS": {Tier: "public", Type: "csv", Default: "", HasDefault: true}},
			want:  ClassifiedVars{Public: []Variable{pub("FLAGS", "")}},
			added: 1,
		},
		{
			name:  "environment value wins over the default",
			vars:  ClassifiedVars{Public: []Variable{pub("FLAGS", "from-env")}},
			decls: map[string]*manifest.VarDecl{"FLAGS": {Tier: "public", Type: "csv", Default: "a,b", HasDefault: true}},
			want:  ClassifiedVars{Public: []Variable{pub("FLAGS", "from-env")}},
		},
		{
			name: "a value set in another tier also wins, so no collision is created",
			vars: ClassifiedVars{Server: []Variable{mk(TierServer, "FLAGS", "s")}},
			decls: map[string]*manifest.VarDecl{
				"FLAGS": {Tier: "public", Type: "csv", Default: "a,b", HasDefault: true},
			},
			want: ClassifiedVars{Server: []Variable{mk(TierServer, "FLAGS", "s")}},
		},
		{
			name:  "required variable is never defaulted",
			decls: map[string]*manifest.VarDecl{"ENV": {Tier: "public", Required: true, Default: "prod", HasDefault: true}},
		},
		{
			name:  "no default declared means nothing is added",
			decls: map[string]*manifest.VarDecl{"OPTIONAL": {Tier: "public", Type: "string"}},
		},
		{
			name: "defaults land in the tier they declare",
			decls: map[string]*manifest.VarDecl{
				"REGION":   {Tier: "sensitive", Type: "string", Default: "eu", HasDefault: true},
				"UPSTREAM": {Tier: "server", Type: "string", Default: "api:80", HasDefault: true},
			},
			want: ClassifiedVars{
				Sensitive: []Variable{mk(TierSensitive, "REGION", "eu")},
				Server:    []Variable{mk(TierServer, "UPSTREAM", "api:80")},
			},
			added: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := tt.vars
			added := vars.ApplyDefaults(&manifest.Manifest{Variables: tt.decls})
			if len(added) != tt.added {
				t.Errorf("added %d variables, want %d: %+v", len(added), tt.added, added)
			}
			if !reflect.DeepEqual(vars, tt.want) {
				t.Errorf("vars = %+v, want %+v", vars, tt.want)
			}
		})
	}
}

func TestApplyDefaults_NilManifest(t *testing.T) {
	var vars ClassifiedVars
	if added := vars.ApplyDefaults(nil); added != nil {
		t.Fatalf("ApplyDefaults(nil) = %v, want nil", added)
	}
}
