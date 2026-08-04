package result

import (
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

type Envelope struct {
	Success bool           `json:"success"`
	Service string         `json:"service"`
	Tool    string         `json:"tool"`
	Data    any            `json:"data,omitempty"`
	Error   *Error         `json:"error,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type Error struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	HTTPCode int    `json:"httpStatus,omitempty"`
	Detail   any    `json:"detail,omitempty"`
}

func OK(service, tool string, data any) Envelope {
	return Envelope{Success: true, Service: service, Tool: tool, Data: data}
}

func Fail(service, tool, code, message string) Envelope {
	return Envelope{Success: false, Service: service, Tool: tool, Error: &Error{Code: code, Message: message}}
}

func FailHTTP(service, tool, code, message string, status int) Envelope {
	return FailHTTPDetail(service, tool, code, message, status, nil)
}

func FailHTTPDetail(service, tool, code, message string, status int, detail any) Envelope {
	return Envelope{Success: false, Service: service, Tool: tool, Error: &Error{Code: code, Message: message, HTTPCode: status, Detail: detail}}
}

// MustOutputSchema builds the JSON Schema every tool uses for its Envelope-shaped tool result.
//
// github.com/google/jsonschema-go infers an empty (all-zero) Schema for the "any"-typed Data and
// Error.Detail fields, and its Schema.MarshalJSON collapses an all-empty Schema to the JSON Schema
// boolean literal `true` (spec-valid shorthand for "matches anything"). Some MCP clients, including
// Claude Code, reject that boolean form inside outputSchema.properties with "Invalid input" and never
// list the tool. Giving each field a Description avoids the empty-schema case while staying just as
// permissive.
//
// Panics on failure, matching mcp.AddTool's own behavior for schema errors: Envelope's shape is fixed
// at compile time, so this can only fail if a future edit makes it unsupported by reflection.
func MustOutputSchema() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[Envelope](), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("result: building Envelope output schema: %v", err))
	}
	if data, ok := schema.Properties["data"]; ok {
		data.Description = "Tool-specific result payload; shape varies by tool."
	}
	if errProp, ok := schema.Properties["error"]; ok {
		if detail, ok := errProp.Properties["detail"]; ok {
			detail.Description = "Additional machine-readable error context; shape varies by error."
		}
	}
	return schema
}
