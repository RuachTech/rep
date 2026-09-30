package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ruachtech/rep/gateway/internal/manifest"
)

func TestApplyDefaults(t *testing.T) {
	mk := func(tier Tier, name, value string) Variable {
		return Variable{Name: name, Value: value, Tier: tier, OriginalKey: tier.Prefix() + name}
	}
	pub := func(name, value string) Variable { return mk(TierPublic, name, value) }

	tests := []struct {
		name    string
		vars    ClassifiedVars
		decls   map[string]*manifest.VarDecl
		wantErr string
		want    ClassifiedVars
		added   int
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
		{
			name:    "a default must satisfy the declared type",
			decls:   map[string]*manifest.VarDecl{"TIMEOUT": {Tier: "public", Type: "number", Default: "soon", HasDefault: true}},
			wantErr: `default: variable "TIMEOUT" must be a number`,
		},
		{
			name:    "a default must satisfy the declared pattern",
			decls:   map[string]*manifest.VarDecl{"CODE": {Tier: "public", Type: "string", Pattern: `[A-Z]{3}`, Default: "abc", HasDefault: true}},
			wantErr: `default: variable "CODE" value does not match pattern`,
		},
		{
			name:    "an invalid default is refused even while the environment overrides it",
			vars:    ClassifiedVars{Public: []Variable{pub("TIMEOUT", "30")}},
			decls:   map[string]*manifest.VarDecl{"TIMEOUT": {Tier: "public", Type: "number", Default: "soon", HasDefault: true}},
			wantErr: `default: variable "TIMEOUT" must be a number`,
		},
		{
			name:    "a default with no valid tier is refused",
			decls:   map[string]*manifest.VarDecl{"FLAGS": {Tier: "", Type: "csv", Default: "", HasDefault: true}},
			wantErr: `variable "FLAGS" declares a default but its tier "" is not public, sensitive or server`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := tt.vars
			added, err := vars.ApplyDefaults(&manifest.Manifest{Variables: tt.decls})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ApplyDefaults() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyDefaults() unexpected error: %v", err)
			}
			if len(added) != tt.added {
				t.Errorf("added %d variables, want %d: %+v", len(added), tt.added, added)
			}
			if !reflect.DeepEqual(vars, tt.want) {
				t.Errorf("vars = %+v, want %+v", vars, tt.want)
			}
		})
	}
}

// On error the receiver is left untouched, even by defaults that were valid.
func TestApplyDefaults_ErrorAddsNothing(t *testing.T) {
	var vars ClassifiedVars
	_, err := vars.ApplyDefaults(&manifest.Manifest{Variables: map[string]*manifest.VarDecl{
		"A_FLAGS": {Tier: "public", Type: "csv", Default: "ok", HasDefault: true},
		"B_PORT":  {Tier: "public", Type: "number", Default: "nope", HasDefault: true},
	}})
	if err == nil {
		t.Fatal("ApplyDefaults() error = nil, want an invalid-default error")
	}
	if !reflect.DeepEqual(vars, ClassifiedVars{}) {
		t.Errorf("vars = %+v after an error, want them untouched", vars)
	}
}

func TestApplyDefaults_NilManifest(t *testing.T) {
	var vars ClassifiedVars
	added, err := vars.ApplyDefaults(nil)
	if err != nil || added != nil {
		t.Fatalf("ApplyDefaults(nil) = %v, %v; want nil, nil", added, err)
	}
}
