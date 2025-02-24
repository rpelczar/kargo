package directives

import (
	"fmt"
)

func Register(directive Promoter) {
	directives.register(directive)
}

// directives is a registry of directives.
var directives = directivesRegistry{}

// directivesRegistry is a registry of directives.
type directivesRegistry map[string]Promoter

// register registers a directive with the given name. If a directive with the
// same name has already been registered, it will be overwritten. The directive
// must implement the Namer interface.
func (d directivesRegistry) register(directive Promoter) {
	d[directive.Name()] = directive
}

// GetPromoter returns the Promoter with the specified name. It returns an error
// if no directive with the specified name has been registered, or if the
// registered directive does not implement the Promoter interface.
func (d directivesRegistry) GetPromoter(name string) (Promoter, error) {
	promoter, ok := d[name]
	if !ok {
		return nil, fmt.Errorf("directive %q not found", name)
	}
	return promoter, nil
}

// GetHealthChecker returns the HealthChecker with the specified name. It
// returns an error if no directive with the specified name has been registered,
// or if the registered directive does not implement the HealthChecker
// interface.
func (d directivesRegistry) GetHealthChecker(name string) (HealthChecker, error) {
	directive, ok := d[name]
	if !ok {
		return nil, fmt.Errorf("directive %q not found", name)
	}
	healthChecker, ok := directive.(HealthChecker)
	if !ok {
		return nil, fmt.Errorf("directive %q does not implement HealthChecker", name)
	}
	return healthChecker, nil
}
