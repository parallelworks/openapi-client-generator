package analyzer

import (
	"fmt"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

// analyzeWebhooks lowers the payloads the API sends: the document's webhooks and
// every operation's callbacks. Both are requests the caller receives, so what a
// generated client can offer is the type to decode them into.
func (a *Analyzer) analyzeWebhooks(pkg *ir.Package) {
	if a.model.Webhooks == nil {
		return
	}
	for name, pathItem := range a.model.Webhooks.FromOldest() {
		if pathItem == nil {
			continue
		}
		defs, warnings := a.inboundPayloads(name, naming.Exported(name), false, pathItem)
		pkg.Webhooks = append(pkg.Webhooks, defs...)
		pkg.Warnings = append(pkg.Warnings, warnings...)
	}
}

// callbackPayloads lowers one operation's callbacks. The key a receiver
// dispatches on is the operation and callback name, since a callback arrives at
// a URL the caller registered rather than under a name of its own.
func (a *Analyzer) callbackPayloads(opName string, op *v3high.Operation) ([]*ir.WebhookDef, []string) {
	if op.Callbacks == nil {
		return nil, nil
	}
	var defs []*ir.WebhookDef
	var warnings []string
	for callbackName, callback := range op.Callbacks.FromOldest() {
		if callback == nil || callback.Expression == nil {
			continue
		}
		for _, pathItem := range callback.Expression.FromOldest() {
			if pathItem == nil {
				continue
			}
			name := opName + "." + callbackName
			goName := opName + naming.Exported(callbackName)
			callbackDefs, callbackWarnings := a.inboundPayloads(name, goName, true, pathItem)
			defs = append(defs, callbackDefs...)
			warnings = append(warnings, callbackWarnings...)
		}
	}
	return defs, warnings
}

// inboundPayloads converts the operations of one webhook or callback path item.
// A path item with several methods names each one, since they carry different
// payloads.
func (a *Analyzer) inboundPayloads(name, goName string, callback bool, pathItem *v3high.PathItem) ([]*ir.WebhookDef, []string) {
	ops := pathOperations(pathItem)
	var defs []*ir.WebhookDef
	var warnings []string
	for _, m := range ops {
		methodName, methodKey := goName, name
		if len(ops) > 1 {
			methodName += naming.Exported(m.method)
			methodKey += "." + m.method
		}

		// Two spec keys can normalize to one Go name, and the parse functions
		// built from it share the package scope.
		methodName = a.namer.Unique(methodName)

		payloadType, ok := a.inboundPayloadType(m.op, methodName)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s %q: no JSON request body to decode, so no parse function is generated", inboundKind(callback), methodKey))
			continue
		}

		defs = append(defs, &ir.WebhookDef{
			Name:        methodKey,
			GoName:      methodName,
			Callback:    callback,
			Method:      m.method,
			PayloadType: payloadType,
			Description: m.op.Description,
		})
	}
	return defs, warnings
}

// inboundPayloadType returns the Go type a webhook or callback body decodes
// into, reporting false when there is no JSON body to decode.
func (a *Analyzer) inboundPayloadType(op *v3high.Operation, nameHint string) (string, bool) {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return "", false
	}
	contentType, mediaType := preferredContent(op.RequestBody.Content)
	if !strings.Contains(contentType, "json") || mediaType == nil {
		return "", false
	}
	end := a.enterBodyScope(nameHint+"Payload", op.RequestBody.GoLow().IsReference())
	goType := a.resolveMediaTypeSchema(mediaType, nameHint+"Payload")
	end()
	if goType == "" || goType == "any" {
		return "", false
	}
	return goType, true
}

func inboundKind(callback bool) string {
	if callback {
		return "callback"
	}
	return "webhook"
}
