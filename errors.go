package acyclicloader

import (
	"fmt"
	"strings"
)

// A DependencyLoadError indicates that a dependency of a component failed to load.
type DependencyLoadError struct {
	trace []string
	err   error
}

func (e *DependencyLoadError) extend(component string) error {
	return &DependencyLoadError{
		trace: append([]string{component}, e.trace...),
		err:   e.err,
	}
}

func (e *DependencyLoadError) Error() string {
	return fmt.Sprintf("failed to load dependency %s: %s", strings.Join(e.trace, " -> "), e.err)
}

// An UndefinedComponentError indicates that Load() was given a component which
// wasn't defined.
type UndefinedComponentError struct {
	Component string
}

func (e *UndefinedComponentError) Error() string {
	return fmt.Sprintf("cannot load undefined component '%s'", e.Component)
}

// A ComponentDefinitionError is returned if the definition of components
// contains a bug such as type error, dependency cycle or unknown dependency.
type ComponentDefinitionError struct {
	Component string
	message   string
}

func (e *ComponentDefinitionError) Error() string {
	return e.message
}
