package randregex

import (
	"math/rand/v2"
	"regexp"
	"regexp/syntax"
	"strconv"
	"sync"
	"testing"
)

func TestGeneratedSamplesMatch(t *testing.T) {
	tests := []struct {
		name   string
		pat    string
		repeat int
	}{
		{name: "empty pattern", pat: ``},
		{name: "literal string", pat: `hello`},
		{name: "escaped literals", pat: `a\+b\?c\.`},
		{name: "unicode literal", pat: `café`},
		{name: "concatenation", pat: `[a-z][0-9]x`},
		{name: "alternation", pat: `foo|bar`},
		{name: "nested alternation", pat: `(foo|bar)-(baz|qux)`},
		{name: "capturing group", pat: `(ab|cd)[0-9]`},
		{name: "non-capturing group", pat: `(?:ab|cd)[0-9]`},
		{name: "class lowercase", pat: `[a-z]`},
		{name: "class wordish", pat: `[a-zA-Z0-9_]`},
		{name: "predefined digit", pat: `\d`},
		{name: "predefined word", pat: `\w`},
		{name: "predefined space", pat: `\s`},
		{name: "negated digit", pat: `\D`},
		{name: "negated word", pat: `\W`},
		{name: "negated space", pat: `\S`},
		{name: "dot", pat: `.`},
		{name: "optional", pat: `ab?c`},
		{name: "star", pat: `ab*c`},
		{name: "plus", pat: `ab+c`},
		{name: "exact repetition", pat: `[a-z]{4}`},
		{name: "bounded repetition", pat: `[a-z]{2,8}`},
		{name: "unbounded repetition", pat: `[a-z]{3,}`},
		{name: "line anchors", pat: `^[a-z]{4}$`},
		{name: "text anchors", pat: `\A[a-z]{4}\z`},
		{name: "word boundary anchors", pat: `\b[a-z]{4}\b`},
		{name: "non-word boundary anchor", pat: `[a-z]\B[a-z]`},
		{name: "max repeat zero star", pat: `a*`, repeat: 0},
		{name: "max repeat zero plus", pat: `a+`, repeat: 0},
		{name: "min above max repeat", pat: `a{40,}`, repeat: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxRepeat := DefaultMaxRepeat
			if tt.repeat != 0 || tt.name == "max repeat zero star" || tt.name == "max repeat zero plus" {
				maxRepeat = tt.repeat
			}
			g, err := Compile(tt.pat, maxRepeat)
			if err != nil {
				t.Fatalf("Compile(%q): %v", tt.pat, err)
			}
			re := regexp.MustCompile(`^(?:` + tt.pat + `)$`)
			r := rand.New(rand.NewPCG(1, 2))
			for i := 0; i < 1000; i++ {
				s := g.StringWithRand(r)
				if !re.MatchString(s) {
					t.Fatalf("sample %d for %q did not match: %q", i, tt.pat, s)
				}
			}
		})
	}
}

func TestAppendAppendsToExistingBuffer(t *testing.T) {
	g := MustCompile(`[a-z]{8}`, DefaultMaxRepeat)
	r := rand.New(rand.NewPCG(1, 2))
	dst := []byte("prefix:")
	got := g.AppendWithRand(dst, r)
	if string(got[:len(dst)]) != "prefix:" {
		t.Fatalf("prefix was not preserved: %q", got)
	}
	if !regexp.MustCompile(`^prefix:[a-z]{8}$`).Match(got) {
		t.Fatalf("appended output did not match: %q", got)
	}
}

func TestInvalidRegexPattern(t *testing.T) {
	if _, err := Compile(`[`, DefaultMaxRepeat); err == nil {
		t.Fatal("Compile returned nil error for invalid pattern")
	}
}

func TestNegativeMaxRepeat(t *testing.T) {
	if _, err := Compile(`a`, -1); err == nil {
		t.Fatal("Compile returned nil error for negative maxRepeat")
	}
}

func TestUnsupportedNoMatch(t *testing.T) {
	_, err := FromRegexp(&syntax.Regexp{Op: syntax.OpNoMatch}, DefaultMaxRepeat)
	if err == nil {
		t.Fatal("FromRegexp returned nil error for OpNoMatch")
	}
}

func TestUnsupportedUnsampleableClass(t *testing.T) {
	_, err := Compile(`[^\s\S]`, DefaultMaxRepeat)
	if err == nil {
		t.Fatal("Compile returned nil error for unsampleable class")
	}
}

func TestWordBoundaryMustBeGuaranteed(t *testing.T) {
	tests := []string{
		`\b`,
		`\b|a`,
		`a?\b`,
		`\B[a-z]`,
		`[a-z]\b[a-z]`,
	}

	for _, pat := range tests {
		t.Run(pat, func(t *testing.T) {
			if _, err := Compile(pat, DefaultMaxRepeat); err == nil {
				t.Fatal("Compile returned nil error for non-guaranteed word boundary")
			}
		})
	}
}

func TestRootNoWordBoundary(t *testing.T) {
	g := MustCompile(`\B`, DefaultMaxRepeat)
	got := g.String()
	if got != "" {
		t.Fatalf(`\B generated %q, want empty string`, got)
	}
	if !regexp.MustCompile(`^(?:\B)$`).MatchString(got) {
		t.Fatal(`empty string did not match \B`)
	}
}

func TestMaxRepeatSemantics(t *testing.T) {
	tests := []struct {
		pat       string
		maxRepeat int
		wantLen   int
	}{
		{pat: `a*`, maxRepeat: 0, wantLen: 0},
		{pat: `a+`, maxRepeat: 0, wantLen: 1},
		{pat: `a{3,}`, maxRepeat: 0, wantLen: 3},
		{pat: `a{40,}`, maxRepeat: 32, wantLen: 40},
	}

	for _, tt := range tests {
		t.Run(tt.pat, func(t *testing.T) {
			g := MustCompile(tt.pat, tt.maxRepeat)
			got := g.StringWithRand(rand.New(rand.NewPCG(1, 2)))
			if len(got) != tt.wantLen {
				t.Fatalf("len(%q) = %d, want %d", got, len(got), tt.wantLen)
			}
		})
	}
}

func TestFromRegexpDoesNotMutate(t *testing.T) {
	re, err := syntax.Parse(`[a-z]{3,}`, syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	before := re.String()
	if _, err := FromRegexp(re, DefaultMaxRepeat); err != nil {
		t.Fatal(err)
	}
	if after := re.String(); after != before {
		t.Fatalf("FromRegexp mutated regexp: before %q, after %q", before, after)
	}
}

func TestDeterministicOutputWithSeededRand(t *testing.T) {
	g := MustCompile(`[a-z]{8}\d{2}`, DefaultMaxRepeat)
	r1 := rand.New(rand.NewPCG(1, 2))
	r2 := rand.New(rand.NewPCG(1, 2))

	for i := 0; i < 100; i++ {
		s1 := g.StringWithRand(r1)
		s2 := g.StringWithRand(r2)
		if s1 != s2 {
			t.Fatalf("sequence diverged at %d: %q != %q", i, s1, s2)
		}
	}
}

func TestCryptoRandGeneratesMatchingSamples(t *testing.T) {
	var r Rand = CryptoRand
	g := MustCompile(`[a-zA-Z0-9_-]{32}`, DefaultMaxRepeat)
	re := regexp.MustCompile(`^(?:[a-zA-Z0-9_-]{32})$`)

	for i := 0; i < 100; i++ {
		s := g.StringWithRand(r)
		if !re.MatchString(s) {
			t.Fatalf("generated invalid sample: %q", s)
		}
	}
}

func TestCryptoRandIntNRange(t *testing.T) {
	tests := []int{1, 2, 10, 95, 256, 257, 1000, 1 << 20}
	if strconv.IntSize == 64 {
		tests = append(tests, 1<<40)
	}

	for _, n := range tests {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			for range 100 {
				got := CryptoRand.IntN(n)
				if got < 0 || got >= n {
					t.Fatalf("CryptoRand.IntN(%d) = %d, want [0, %d)", n, got, n)
				}
			}
		})
	}
}

func TestCryptoRandPanicsForNonPositiveN(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("CryptoRand.IntN did not panic")
		}
	}()
	CryptoRand.IntN(0)
}

func TestConcurrentUse(t *testing.T) {
	g := MustCompile(`(foo|bar|baz)-[0-9]{4}`, DefaultMaxRepeat)
	re := regexp.MustCompile(`^(?:(foo|bar|baz)-[0-9]{4})$`)

	var wg sync.WaitGroup
	for id := 0; id < 16; id++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed+1))
			for i := 0; i < 1000; i++ {
				s := g.StringWithRand(r)
				if !re.MatchString(s) {
					t.Errorf("generated invalid sample: %q", s)
					return
				}
			}
		}(uint64(id + 1))
	}
	wg.Wait()
}

func TestConcurrentDefaultRandUse(t *testing.T) {
	g := MustCompile(`[a-z0-9]{32}`, DefaultMaxRepeat)
	re := regexp.MustCompile(`^(?:[a-z0-9]{32})$`)

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				s := g.String()
				if !re.MatchString(s) {
					t.Errorf("generated invalid sample: %q", s)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentCryptoRandUse(t *testing.T) {
	g := MustCompile(`[a-z0-9]{32}`, DefaultMaxRepeat)
	re := regexp.MustCompile(`^(?:[a-z0-9]{32})$`)

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				s := g.StringWithRand(CryptoRand)
				if !re.MatchString(s) {
					t.Errorf("generated invalid sample: %q", s)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestMustCompilePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustCompile did not panic")
		}
	}()
	_ = MustCompile(`[`, DefaultMaxRepeat)
}
