Acyclic Component Loader for Go [![Build Status](https://travis-ci.org/jonasfj/go-acyclicloader.svg?branch=master)](https://travis-ci.org/jonasfj/go-acyclicloader)
===============================

The `acyclicloader` package creates an object that can load components, from
a set of acyclic dependent components. Each component is defined with name
and a function that loads the component. The function that loads the component
may take a single struct as argument, fields on the struct defines dependency
on other components.

```go
import (
  "fmt"
  "net/http"
  "github.com/jonasfj/go-acyclicloader"
)

var loader = acyclicloader.Components{
    "Port": func() int { return 80 },
    "Server": func(options struct {
        Port int // The 'Server' component depends on the 'Port' component
    }) *http.Server {
        return &http.Server{Port: fmt.Sprintf(":%d", options.Port)}
    }
}.AsLoader()

func main() {
    s := loader.MustLoad("Server").(*http.Server)
    s.ListenAndServe()
}
```

When loading a component the loader will load all dependencies concurrently.
As an added benefit you can overwrite component using
`loader.WithOverwrites(map[string]interface{}{"Port": 8080})`, which is
useful when writing tests. For more details refer to the example or
documentation:

 * [Documentation](https://godoc.org/github.com/jonasfj/go-acyclicloader)
 * [Example](https://github.com/jonasfj/go-acyclicloader/blob/master/example_loader_test.go)

License
-------
This package is released under [MPLv2](https://www.mozilla.org/en-US/MPL/2.0/).
