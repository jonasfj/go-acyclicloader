// Package acyclicloader provides a construct for declaring a directed-acyclic
// graph of dependent components and loading them with maximum concurrency.
//
// This aims to make it easy to stitch together a bunch of dependent components,
// while reusing the same loading code in tests, as individual components can
// be overwritten with mock objects. For details see the example.
//
//   var loader = acyclicloader.Components{
//       "Port": func() int { return 80 },
//       "Server": func(options struct {
//           Port int // The 'Server' component depends on the 'Port' component
//       }) *http.Server {
//           return &http.Server{Port: fmt.Sprintf(":%d", options.Port)}
//       }
//   }.AsLoader()
//
//   func main() {
//       s := loader.MustLoad("Server").(*http.Server)
//       s.ListenAndServe()
//   }
package acyclicloader
