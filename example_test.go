package randregex_test

import (
	"fmt"
	"math/rand/v2"

	"github.com/ryanfowler/randregex"
)

func ExampleCompile() {
	g, err := randregex.Compile(`[a-z]{8}\d{2}`)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(g.String()))
	// Output: 10
}

func ExampleCompileMaxRepeat() {
	g, err := randregex.CompileMaxRepeat(`a{4,}`, 4)
	if err != nil {
		panic(err)
	}

	fmt.Println(g.String())
	// Output: aaaa
}

func ExampleGenerator_StringWithRand() {
	r := rand.New(rand.NewPCG(1, 2))
	g := randregex.MustCompile(`[a-z]{8}`)

	fmt.Println(g.StringWithRand(r))
	// Output: uquugbml
}

func ExampleCryptoRand() {
	g := randregex.MustCompile(`[A-F0-9]{16}`)

	fmt.Println(len(g.StringWithRand(randregex.CryptoRand)))
	// Output: 16
}

func ExampleGenerator_Append() {
	g := randregex.MustCompile(`[a-zA-Z0-9_-]{24}`)
	buf := make([]byte, 0, 64)

	buf = g.Append(buf)

	fmt.Println(len(buf))
	// Output: 24
}
