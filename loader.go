package acyclicloader

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// An AcyclicLoader holds functions for loading components with acyclic
// dependencies with maximum concurrency.
type AcyclicLoader struct {
	m          sync.Mutex
	c          sync.Cond
	components map[string]*component
}

type component struct {
	fn           reflect.Value
	result       reflect.Type
	dependencies []string
	value        interface{}
	err          error
	loaded       bool
	loading      bool
}

// Components holds a set of components with acyclic inter-dependencies.
//
// A component is a mapping from a string to function that loads the component.
// The right-hand-side in this mapping must be a function on the form:
//   func () ComponentType
//   func () (ComponentType, error)
//   func (struct{Dependency DependencyType, ...}) ComponentType
//   func (struct{Dependency DependencyType, ...}) (ComponentType, error)
// where ComponentType is the type of the component, and Dendency is a component
// that this component depends on and DependencyType is the type of said
// dependency.
//
// For example, the following "Users" component has type *UserModel and depends
// on the "Database" component which has the type *sql.DB.
//   "Users": func(options struct { Database *sql.DB }) *UserModel {
//       return &UserModel{db: options.Database}
//   },
type Components map[string]interface{}

// AsLoader returns an AcyclicLoader or panics
func (c Components) AsLoader() *AcyclicLoader {
	a, err := New(c)
	if err != nil {
		panic(err)
	}
	return a
}

// MustLoad will load given component or panics
func (c Components) MustLoad(component string) interface{} {
	v, err := c.AsLoader().Load(component)
	if err != nil {
		panic(err)
	}
	return v
}

// New creates a AcyclicLoader from a set of components.
//
// This will return an error if there is some type error, cyclic dependency or
// missing dependency in the set of components given. Since such an error is
// consistent it is preferable to use acyclicloader.Components{...}.AsLoader()
// when creating a loader as global variable.
func New(components Components) (*AcyclicLoader, error) {
	a := &AcyclicLoader{
		components: make(map[string]*component, len(components)),
	}
	a.c.L = &a.m

	// Sort component names so that the error returned is always the same
	// otherwise it gets really confusing to debug
	componentNames := make([]string, 0, len(components))
	for name := range components {
		componentNames = append(componentNames, name)
	}
	sort.Strings(componentNames)

	// Populate components
	for _, name := range componentNames {
		fn := components[name]
		if fn == nil {
			return nil, &ComponentDefinitionError{
				Component: name,
				message:   fmt.Sprintf("expected definition of '%s' to be a function, but found nil", name),
			}
		}
		t := reflect.TypeOf(fn)
		if t.Kind() != reflect.Func {
			return nil, &ComponentDefinitionError{
				Component: name,
				message: fmt.Sprintf(
					"expected definition of '%s' to be a function, but found %s",
					name, t.String(),
				),
			}
		}
		var result reflect.Type
		switch t.NumOut() {
		case 0:
		case 1:
			if t.Out(0) != typeOfError {
				result = t.Out(0)
			}
		case 2:
			result = t.Out(0)
			if t.Out(1) != typeOfError {
				return nil, &ComponentDefinitionError{
					Component: name,
					message: fmt.Sprintf(
						"expected 2nd result from '%s' to have error type, but found %s",
						name, t.Out(1).String(),
					),
				}
			}
		default:
			return nil, &ComponentDefinitionError{
				Component: name,
				message: fmt.Sprintf(
					"expected no more than 2 results from '%s', but found %d outputs",
					name, t.NumOut(),
				),
			}
		}
		a.components[name] = &component{
			fn:     reflect.ValueOf(fn),
			result: result,
		}
	}

	// Populate and check dependencies
	for _, name := range componentNames {
		component := a.components[name]
		t := component.fn.Type()
		switch t.NumIn() {
		case 0:
			continue // dependencies = nil
		case 1:
			// We continue below
		default:
			return nil, &ComponentDefinitionError{
				Component: name,
				message: fmt.Sprintf(
					"expected no more than 1 input parameter for '%s', but found %d",
					name, t.NumIn(),
				),
			}
		}
		input := t.In(0)
		if input.Kind() != reflect.Struct {
			return nil, &ComponentDefinitionError{
				Component: name,
				message: fmt.Sprintf(
					"expected input parameter for '%s' to be a struct, but found %s",
					name, t.In(0).String(),
				),
			}
		}
		component.dependencies = make([]string, 0, input.NumField())
		for i := 0; i < input.NumField(); i++ {
			field := input.Field(i)
			dep, ok := a.components[field.Name]
			if !ok {
				return nil, &ComponentDefinitionError{
					Component: name,
					message: fmt.Sprintf(
						"'%s' depends on undefined component '%s'",
						name, field.Name,
					),
				}
			}
			if dep.result != field.Type {
				return nil, &ComponentDefinitionError{
					Component: name,
					message: fmt.Sprintf(
						"'%s' depends on component '%s' with type %s, but '%s' expects %s",
						name, field.Name, dep.result.String(), name, field.Type.String(),
					),
				}
			}
			component.dependencies = append(component.dependencies, field.Name)
		}
	}

	// Check for cycles
	for _, name := range componentNames {
		component := a.components[name]
		cycle := a.detectCycles(component, []string{name})
		if cycle != nil {
			return nil, &ComponentDefinitionError{
				Component: cycle[0],
				message: fmt.Sprintf(
					"dependency cycle detected: '%s'", strings.Join(cycle, "' -> '"),
				),
			}
		}
	}

	return a, nil
}

func (a *AcyclicLoader) detectCycles(c *component, path []string) []string {
	for _, dep := range c.dependencies {
		for i, name := range path {
			if name == dep {
				return append(path[i:], dep)
			}
		}
		if ret := a.detectCycles(a.components[dep], append(path, dep)); ret != nil {
			return ret
		}
	}
	return nil
}

// WithOverwrites returns an an AcyclicLoader with values overwriting the given
// component names.
func (a *AcyclicLoader) WithOverwrites(values map[string]interface{}) *AcyclicLoader {
	a2 := &AcyclicLoader{
		components: make(map[string]*component, len(a.components)),
	}
	a2.c.L = &a2.m

	// We need to purge any value/err pair that depends on something defined in
	// values, as these are overwritten.
	var needsPurging func(component string) bool
	needsPurging = func(component string) bool {
		if _, ok := values[component]; ok {
			return true
		}
		for _, dep := range a.components[component].dependencies {
			if needsPurging(dep) {
				return true
			}
		}
		return false
	}

	a.m.Lock()
	for name, c := range a.components {
		var value interface{}
		var err error
		if !needsPurging(name) {
			value = c.value
			err = c.err
		}
		if val, ok := values[name]; ok {
			value = val
			err = nil
		}
		a2.components[name] = &component{
			fn:           c.fn,
			result:       c.result,
			dependencies: c.dependencies,
			value:        value,
			err:          err,
			loaded:       c.loaded,
			loading:      c.loaded,
		}
	}
	a.m.Unlock()

	return a2
}

// Clone an AcyclicLoader including cache as far as is currently loaded.
//
// An AcyclicLoader caches loaded components internally, so when a global
// instance in testing it is useful to create a clone of it.
func (a *AcyclicLoader) Clone() *AcyclicLoader {
	a2 := &AcyclicLoader{
		components: make(map[string]*component, len(a.components)),
	}
	a2.c.L = &a2.m

	a.m.Lock()
	defer a.m.Unlock()

	for name, c := range a.components {
		a2.components[name] = &component{
			fn:           c.fn,
			result:       c.result,
			dependencies: c.dependencies,
			value:        c.value,
			err:          c.err,
			loaded:       c.loaded,
			loading:      c.loaded,
		}
	}

	return a2
}

// MustLoad will load given component or panics
func (a *AcyclicLoader) MustLoad(component string) interface{} {
	v, err := a.Load(component)
	if err != nil {
		panic(err)
	}
	return v
}

// Load and cache a given component
//
// An AcyclicLoader will always cache loaded components internally, to reload
// components in testing using the Clone() method to create an AcyclicLoader
// with a separate cache.
func (a *AcyclicLoader) Load(component string) (interface{}, error) {
	a.m.Lock()
	defer a.m.Unlock()

	// Find the component
	c, ok := a.components[component]
	if !ok {
		return nil, &UndefinedComponentError{Component: component}
	}

	// If loaded we're done
	if c.loaded {
		return c.value, c.err
	}

	// Create input argument
	var in []reflect.Value
	var err error
	if len(c.dependencies) > 0 {
		input := reflect.New(c.fn.Type().In(0)).Elem()
		in = []reflect.Value{input}

		// Ensure that we're recursively loading all dependencies
		for _, dep := range c.dependencies {
			if !a.components[dep].loading {
				go a.Load(dep)
			}
		}

		// Wait for dependencies to be loaded
		for i, dep := range c.dependencies {
			for !a.components[dep].loaded {
				a.c.Wait()
			}
			// If there is an error we wrap and break
			err = a.components[dep].err
			if err != nil {
				if e, ok := err.(*DependencyLoadError); ok {
					err = e.extend(component)
				} else {
					err = &DependencyLoadError{
						trace: []string{component, dep},
						err:   err,
					}
				}
				break
			}
			input.Field(i).Set(reflect.ValueOf(a.components[dep].value))
		}
	}

	// Mark c as loading
	c.loading = true

	// Obtain value, if no error so far
	var value interface{}
	if err == nil {
		a.m.Unlock()

		// Call the loader to obtain value and err
		ret := c.fn.Call(in)
		if c.result != nil {
			value = ret[0].Interface()
			if len(ret) > 1 {
				err = ret[1].Interface().(error)
			}
		} else if len(ret) == 1 {
			err = ret[0].Interface().(error)
		}

		a.m.Lock()
	}

	// Set value and inform anyone blocked
	c.loaded = true
	c.value = value
	c.err = err
	a.c.Broadcast()

	return c.value, c.err
}
