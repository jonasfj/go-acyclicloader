package acyclicloader_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/jonasfj/go-acyclicloader"
)

// Define components that make up the application. This design aims to let you
// stitch dependent components together, while loading with maximum concurrency.
var components = acyclicloader.Components{
	// A component is a mapping from name to the function that loads it
	"Port": func() int {
		return 80
	},
	// In addition to the component, you can optionally return an error
	"Database": func() (*sql.DB, error) {
		return sql.Open("mysql", os.Getenv("DATABASE_CONNECTION_STRING"))
	},
	"Template": func() (string, error) {
		data, err := ioutil.ReadFile("template.html")
		return string(data), err
	},
	// Dependency on other components is expressed by taking a struct with
	// required components as argument
	"Handler": func(options struct {
		Template string
		Database *sql.DB
	}) http.Handler {
		// Typically, you would pass options to a custom struct that implements
		// all of your HTTP handlers.
		db := options.Database
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var name string
			db.QueryRow("SELECT name FROM users WHERE age=?", 29).Scan(&name)
			w.WriteHeader(http.StatusOK)
			result := strings.Replace(options.Template, "<name>", name, -1)
			w.Write([]byte(result))
		})
	},
	"Server": func(options struct {
		Port    int
		Handler http.Handler
	}) *http.Server {
		return &http.Server{
			Addr:    fmt.Sprintf(":%d", options.Port),
			Handler: options.Handler,
		}
	},
}.AsLoader() // Create AcyclicLoader or panic in case of type errors

func Example() {
	// This will load "Template" and "Database" concurrently
	server := components.MustLoad("Server").(*http.Server)
	// NOTE: MustLoad will panic, if there is an error. To handle it use Load
	server.ListenAndServe()
}

// If we wanted to write tests we could a specific component
func testSpecificTemplate(t *testing.T) {
	template := components.MustLoad("Template").(string)
	if template == "" {
		t.Fail()
	}
}

// Or we can test the application, while overwriting certain components
func testWithOverwrites(t *testing.T) {
	// Create a database for testing
	db, _ := sql.Open("sqlite", ":memory:")

	// With overwrites we inject values for Port and Database, so that these
	// aren't loaded. This is a great way to inject mock objects.
	server := components.WithOverwrites(map[string]interface{}{
		"Port":     60000, // This port is better when testing
		"Database": db,    // Use a different database in our tests
	}).MustLoad("Server").(*http.Server)

	go server.ListenAndServe()
	defer server.Close()

	// Test that the server works
}
