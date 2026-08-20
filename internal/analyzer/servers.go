package analyzer

import (
	"regexp"
	"strconv"

	naming "github.com/giraffesyo/openapi-go-naming"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// serverTemplateVar matches one {variable} placeholder of a server URL.
var serverTemplateVar = regexp.MustCompile(`\{([^{}]+)\}`)

// convertServers lowers the spec's servers list, pairing each URL with the
// variables it interpolates.
func (a *Analyzer) convertServers() []*ir.ServerDef {
	var servers []*ir.ServerDef
	for _, server := range a.model.Servers {
		if server == nil || server.URL == "" {
			continue
		}
		sd := &ir.ServerDef{
			URL:         server.URL,
			Description: server.Description,
		}
		// The URL drives the order, so the generated parameters read left to
		// right the way the URL does.
		seen := map[string]bool{}
		goNames := map[string]bool{}
		for _, match := range serverTemplateVar.FindAllStringSubmatch(server.URL, -1) {
			name := match[1]
			// One substitution covers every occurrence, so a variable used twice
			// still takes one parameter.
			if seen[name] {
				continue
			}
			seen[name] = true
			sv := &ir.ServerVar{
				Name:   name,
				GoName: uniqueGoName(goNames, naming.Unexported(name)),
			}
			if server.Variables != nil {
				if declared, ok := server.Variables.Get(name); ok && declared != nil {
					sv.Default = declared.Default
					sv.Enum = declared.Enum
					sv.Description = declared.Description
				}
			}
			sd.Variables = append(sd.Variables, sv)
		}
		servers = append(servers, sd)
	}
	return servers
}

// uniqueGoName keeps two variables that normalize to one Go identifier from
// declaring the same parameter twice.
func uniqueGoName(taken map[string]bool, name string) string {
	candidate := name
	for i := 2; taken[candidate]; i++ {
		candidate = name + strconv.Itoa(i)
	}
	taken[candidate] = true
	return candidate
}
