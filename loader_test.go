package acyclicloader

import "testing"

func TestAcyclicLoader(t *testing.T) {
	i := 5
	loader, _ := New(Components{
		"StaticInt": func() int {
			return i
		},
		"Plus7": func(options struct {
			StaticInt int
		}) int {
			return options.StaticInt + 7
		},
	})

	clone := loader.Clone()

	v, _ := loader.Load("Plus7")
	if v.(int) != 12 {
		t.Error("Expected 12")
	}

	// Wont reload
	i = 7
	v, _ = loader.Load("Plus7")
	if v.(int) != 12 {
		t.Error("Expected 12")
	}
	// The clone isn't loaded so it will load from fresh
	v, _ = clone.Load("Plus7")
	if v.(int) != 14 {
		t.Error("Expected 14")
	}

	// We can also overwrite
	v, _ = loader.WithOverwrites(map[string]interface{}{"StaticInt": 7}).Load("Plus7")
	if v.(int) != 14 {
		t.Error("Expected 14")
	}
}

func TestCyclicDependencies(t *testing.T) {
	_, err := New(Components{
		"A": func(options struct{ B int }) int { return options.B + 5 },
		"B": func(options struct{ C int }) int { return options.C + 5 },
		"C": func(options struct{ B int }) int { return options.B + 5 },
	})
	t.Log(err)
	if err == nil {
		t.Error("expected an error")
	}
}

func TestLongCyclicDependencies(t *testing.T) {
	_, err := New(Components{
		"A": func(options struct{ B int }) int { return options.B + 5 },
		"B": func(options struct{ C int }) int { return options.C + 5 },
		"C": func(options struct{ A int }) int { return options.A + 5 },
	})
	t.Log(err)
	if err == nil {
		t.Error("expected an error")
	}
}

func TestSelfDependency(t *testing.T) {
	_, err := New(Components{
		"A": func(options struct{ A int }) int { return options.A + 5 },
	})
	t.Log(err)
	if err == nil {
		t.Error("expected an error")
	}
}

func TestMissingDependency(t *testing.T) {
	_, err := New(Components{
		"A": func(options struct{ B int }) int { return options.B + 5 },
	})
	t.Log(err)
	if err == nil {
		t.Error("expected an error")
	}
}

func TestDependencyTypeMismatch(t *testing.T) {
	_, err := New(Components{
		"A": func(options struct{ B int }) int { return options.B + 5 },
		"B": func() float64 { return 5 },
	})
	t.Log(err)
	if err == nil {
		t.Error("expected an error")
	}
}
