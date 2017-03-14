package acyclicloader

import "testing"

func TestAcyclicLoader(t *testing.T) {

	loader, _ := New(Components{
		"StaticInt": func() int {
			return 5
		},
		"Plus7": func(options struct {
			StaticInt int
		}) int {
			return options.StaticInt + 7
		},
	})

	//loader.Load("Plus7").(int)
	//loader.WithValues(nil)
	//loader.Clone()

	v, _ := loader.Load("Plus7")
	if v.(int) != 12 {
		t.Error("Expected 12")
	}
}
