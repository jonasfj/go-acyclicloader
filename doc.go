// Package acyclicloader provides a construct for declaring a directed-acyclic
// graph of dependent components and loading them with maximum concurrency.
//
// This aims to make it easy to stitch together a bunch of dependent components,
// while reusing the same loading code in tests, as individual components can
// be overwritten with mock objects. For details see the example.
package acyclicloader
