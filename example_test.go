package randregex_test

import (
	"fmt"
	"math/rand/v2"

	"github.com/ryanfowler/randregex"
)

func ExampleCompile() {
	g, err := randregex.Compile(`[a-z]{8}\d{2}`, randregex.DefaultMaxRepeat)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(g.String()))
	// Output: 10
}

func ExampleGenerator_StringWithRand() {
	r := rand.New(rand.NewPCG(1, 2))
	g := randregex.MustCompile(`[a-z]{8}`, randregex.DefaultMaxRepeat)

	fmt.Println(g.StringWithRand(r))
	// Output: uquugbml
}

func ExampleGenerator_Append() {
	g := randregex.MustCompile(`[a-zA-Z0-9_-]{24}`, randregex.DefaultMaxRepeat)
	buf := make([]byte, 0, 64)

	buf = g.Append(buf)

	fmt.Println(len(buf))
	// Output: 24
}
