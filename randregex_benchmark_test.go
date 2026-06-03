package randregex

import (
	"math/rand/v2"
	"strconv"
	"testing"
)

var benchInt int

var benchPatterns = []string{
	`[a-z]{16}`,
	`[a-zA-Z0-9_-]{24}`,
	`(foo|bar|baz)-[0-9]{4}`,
	`[a-z]{6,12}@[a-z]{4,8}\.(com|net|org)`,
	`[a-z0-9]{128}`,
}

func BenchmarkCompile(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			for b.Loop() {
				g, err := Compile(pattern)
				if err != nil {
					b.Fatal(err)
				}
				_ = g
			}
		})
	}
}

func BenchmarkCompileString(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			for b.Loop() {
				g, err := Compile(pattern)
				if err != nil {
					b.Fatal(err)
				}
				_ = g.String()
			}
		})
	}
}

func BenchmarkGeneratorString(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			g := MustCompile(pattern)
			b.ResetTimer()
			for b.Loop() {
				_ = g.String()
			}
		})
	}
}

func BenchmarkGeneratorStringWithRand(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			g := MustCompile(pattern)
			r := rand.New(rand.NewPCG(1, 2))
			b.ResetTimer()
			for b.Loop() {
				_ = g.StringWithRand(r)
			}
		})
	}
}

func BenchmarkGeneratorAppend(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			g := MustCompile(pattern)
			buf := make([]byte, 0, 256)
			b.ResetTimer()
			for b.Loop() {
				buf = buf[:0]
				buf = g.Append(buf)
			}
			_ = buf
		})
	}
}

func BenchmarkCryptoRandIntN(b *testing.B) {
	for _, n := range []int{2, 10, 64, 95, 256, 257, 1000, 1 << 20} {
		b.Run("n="+strconv.Itoa(n), func(b *testing.B) {
			var x int
			for b.Loop() {
				x ^= CryptoRand.IntN(n)
			}
			benchInt = x
		})
	}
}

func BenchmarkGeneratorAppendWithCryptoRand(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			g := MustCompile(pattern)
			buf := make([]byte, 0, 256)
			b.ResetTimer()
			for b.Loop() {
				buf = buf[:0]
				buf = g.AppendWithRand(buf, CryptoRand)
			}
			_ = buf
		})
	}
}

func BenchmarkGeneratorAppendWithRand(b *testing.B) {
	for _, pattern := range benchPatterns {
		b.Run(pattern, func(b *testing.B) {
			g := MustCompile(pattern)
			r := rand.New(rand.NewPCG(1, 2))
			buf := make([]byte, 0, 256)
			b.ResetTimer()
			for b.Loop() {
				buf = buf[:0]
				buf = g.AppendWithRand(buf, r)
			}
			_ = buf
		})
	}
}
