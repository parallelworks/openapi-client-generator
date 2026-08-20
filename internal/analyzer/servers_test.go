package analyzer

import "testing"

const templatedServerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
servers:
  - url: https://{region}.api.example.com/{base_path}/{region}
    description: regional
    variables:
      region: { default: us-east-1, enum: [us-east-1, eu-west-1] }
      base_path: { default: v2 }
  - url: https://staging.api.example.com
paths: {}
`

func TestServers_VariablesFollowTheURL(t *testing.T) {
	pkg, _ := analyzeSpec(t, templatedServerSpec)

	if len(pkg.Servers) != 2 {
		t.Fatalf("servers = %d, want 2", len(pkg.Servers))
	}
	primary := pkg.Servers[0]
	if len(primary.Variables) != 2 {
		t.Fatalf("variables = %+v, want region and base_path once each", primary.Variables)
	}
	// A variable used twice still takes one parameter, and the URL sets the order.
	if primary.Variables[0].Name != "region" || primary.Variables[1].Name != "base_path" {
		t.Errorf("variable order = %q, %q, want region then base_path", primary.Variables[0].Name, primary.Variables[1].Name)
	}
	if primary.Variables[1].GoName != "basePath" {
		t.Errorf("base_path GoName = %q, want basePath", primary.Variables[1].GoName)
	}
	if primary.Variables[0].Default != "us-east-1" || len(primary.Variables[0].Enum) != 2 {
		t.Errorf("region = %+v, want its default and enum carried", primary.Variables[0])
	}
	if len(pkg.Servers[1].Variables) != 0 {
		t.Errorf("staging server variables = %+v, want none", pkg.Servers[1].Variables)
	}
}

const collidingServerVarSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
servers:
  - url: https://example.com/{base_path}/{basePath}/{type}
paths: {}
`

// Two variables can normalize to one Go identifier, and a variable can be named
// for a keyword. Either one declares a parameter the compiler rejects.
func TestServers_VariableNamesStayValidGo(t *testing.T) {
	pkg, _ := analyzeSpec(t, collidingServerVarSpec)

	var names []string
	for _, v := range pkg.Servers[0].Variables {
		names = append(names, v.GoName)
	}
	want := []string{"basePath", "basePath2", "type_"}
	if len(names) != len(want) {
		t.Fatalf("parameter names = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("parameter %d = %q, want %q", i, names[i], w)
		}
	}
}
