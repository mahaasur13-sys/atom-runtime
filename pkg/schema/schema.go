// Package schema — Schema registry (ATOM-SR).
// SR1: All events must include schema_id.
// SR2: Schema must be versioned.
// SR3: WAL recovery MUST validate schema.
package schema

import (
	"fmt"
	"sync"
)

// Schema describes the structure of an event payload.
type Schema struct {
	ID      string
	Version string
	Fields  []Field
}

// Field describes one field in a schema.
type Field struct {
	Name     string
	Type     string // "string", "int", "float", "bool", "object", "array"
	Required bool
}

// Registry tracks all known schemas.
// SR1: All events must reference a registered schema.
type Registry struct {
	mu      sync.RWMutex
	schemas map[string]*Schema // key: schemaID
}

// GlobalRegistry is the single schema authority.
var GlobalRegistry = &Registry{schemas: make(map[string]*Schema)}

// Register adds a schema to the global registry.
func Register(schema *Schema) {
	GlobalRegistry.mu.Lock()
	defer GlobalRegistry.mu.Unlock()
	GlobalRegistry.schemas[schema.ID] = schema
}

// Validate checks an event payload against a registered schema.
// Returns an error if validation fails.
func Validate(schemaID string, payload map[string]interface{}) error {
	GlobalRegistry.mu.RLock()
	s, ok := GlobalRegistry.schemas[schemaID]
	GlobalRegistry.mu.RUnlock()
	if !ok {
		return fmt.Errorf("schema %q not found", schemaID)
	}
	return validateObject(s, payload)
}

func validateObject(schema *Schema, payload map[string]interface{}) error {
	for _, f := range schema.Fields {
		if f.Required {
			if _, ok := payload[f.Name]; !ok {
				return fmt.Errorf("missing required field %q in schema %s", f.Name, schema.ID)
			}
		}
	}
	return nil
}

// MustRegister is a convenient constructor for a schema.
func MustRegister(id, version string, fields []Field) *Schema {
	s := &Schema{ID: id, Version: version, Fields: fields}
	Register(s)
	return s
}

// SchemaExists checks if a schema is registered.
func SchemaExists(id string) bool {
	GlobalRegistry.mu.RLock()
	defer GlobalRegistry.mu.RUnlock()
	_, ok := GlobalRegistry.schemas[id]
	return ok
}
