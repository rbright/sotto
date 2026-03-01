package transcript

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssembleNormalizesWhitespaceTrailingSpaceAndSentenceCase(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{" hello", "world.", "\nfrom", "sotto"}, Options{
		TrailingSpace:       true,
		CapitalizeSentences: true,
	})
	require.Equal(t, "Hello world. From sotto ", got)
}

func TestAssembleWithoutTrailingSpace(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"hello", "world"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: false,
	})
	require.Equal(t, "hello world", got)
}

func TestAssembleEmptyInput(t *testing.T) {
	t.Parallel()

	require.Empty(t, Assemble(nil, Options{TrailingSpace: true, CapitalizeSentences: true}))
}

func TestAssembleSkipsWhitespaceOnlySegments(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"  ", "\n\t", "hello"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "Hello", got)
}

func TestAssembleSentenceCaseCapitalizesPronounI(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"when i speak i'm clearer. i think i will keep using it."}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "When I speak I'm clearer. I think I will keep using it.", got)
}

func TestAssembleSentenceCaseDoesNotCapitalizeDomainOrDecimalFragments(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"check example.com and v2.1 first. then reply"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "Check example.com and v2.1 first. Then reply", got)
}

func TestAssembleSentenceCaseAbbreviationRegressionCorpus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "measurement_tbsp",
			in:   "add 1 tbsp. sugar and stir",
			want: "Add 1 tbsp. sugar and stir",
		},
		{
			name: "measurement_min",
			in:   "mix for 5 min. then serve",
			want: "Mix for 5 min. then serve",
		},
		{
			name: "title_abbreviation_inside_sentence",
			in:   "we spoke with dr. smith yesterday. then we left",
			want: "We spoke with dr. smith yesterday. Then we left",
		},
		{
			name: "ambiguous_etc_sentence_starter",
			in:   "we covered apples, etc. then moved on",
			want: "We covered apples, etc. Then moved on",
		},
		{
			name: "ambiguous_etc_pronoun_boundary",
			in:   "we listed apples, etc. we moved on",
			want: "We listed apples, etc. We moved on",
		},
		{
			name: "ambiguous_etc_conjunction_continuation",
			in:   "bring apples etc. and bananas",
			want: "Bring apples etc. and bananas",
		},
		{
			name: "ambiguous_vs_conservative",
			in:   "compare this vs. that option. then decide",
			want: "Compare this vs. that option. Then decide",
		},
		{
			name: "initialism_sentence_starter",
			in:   "we moved to the u.s. then we celebrated",
			want: "We moved to the u.s. Then we celebrated",
		},
		{
			name: "initialism_pronoun_boundary",
			in:   "we moved to the u.s. we celebrated",
			want: "We moved to the u.s. We celebrated",
		},
		{
			name: "initialism_embedded_locative_pronoun_boundary",
			in:   "i lived in the u.s. we can travel there",
			want: "I lived in the u.s. We can travel there",
		},
		{
			name: "initialism_conjunction_continuation",
			in:   "we moved to the u.s. and stayed",
			want: "We moved to the u.s. and stayed",
		},
		{
			name: "initialism_locative_non_boundary",
			in:   "in the u.s. we have states",
			want: "In the u.s. we have states",
		},
		{
			name: "initialism_locative_non_boundary_after_sentence_boundary",
			in:   "this is true. in the u.s. we have states",
			want: "This is true. In the u.s. we have states",
		},
		{
			name: "initialism_origin_non_boundary",
			in:   "from the u.s. we have exports",
			want: "From the u.s. we have exports",
		},
		{
			name: "initialism_embedded_origin_boundary",
			in:   "i came from the u.s. we celebrated",
			want: "I came from the u.s. We celebrated",
		},
		{
			name: "no_default_boundary",
			in:   "no. then we continue",
			want: "No. Then we continue",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Assemble([]string{tc.in}, Options{
				TrailingSpace:       false,
				CapitalizeSentences: true,
			})
			require.Equal(t, tc.want, got)
		})
	}
}

func TestAssembleSentenceCaseDoesNotCapitalizeAfterCommonAbbreviations(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"for i.e. this case and e.g. that case. then proceed"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "For i.e. this case and e.g. that case. Then proceed", got)
}

func TestAssembleSentenceCaseKeepsPronounIDistinctFromIEAbbreviation(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"i said i.e. this should stay lowercase"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "I said i.e. this should stay lowercase", got)
}

func TestAssembleSentenceCaseKeepsLeadingIEAbbreviationLowercase(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"i.e. this should stay lowercase"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "i.e. this should stay lowercase", got)
}

func TestAssembleSentenceCaseKeepsPostBoundaryIEAbbreviationLowercase(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"this is true. i.e. this should stay lowercase"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "This is true. i.e. this should stay lowercase", got)
}

func TestAssembleSentenceCaseCapitalizesTitleAbbreviationAtSentenceStart(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"dr. smith can help"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "Dr. smith can help", got)
}

func TestAssembleSentenceCaseCapitalizesTitleAbbreviationAfterBoundary(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"this happened. dr. smith replied"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "This happened. Dr. smith replied", got)
}

func TestAssembleSentenceCaseDoesNotCapitalizeAfterInitialismAbbreviation(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"in the u.s. government report. then we continue"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "In the u.s. government report. Then we continue", got)
}

func TestAssembleSentenceCaseHandlesQuoteAfterBoundary(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"he said. \"hello there\" and left."}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "He said. \"Hello there\" and left.", got)
}

func TestAssembleSentenceCaseLeadingBoundaryDoesNotDoubleCapitalize(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"2. hello there"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, "2. Hello there", got)
}

func TestAssembleIdempotentForNormalizedOutput(t *testing.T) {
	t.Parallel()

	first := Assemble([]string{"hello world. this is sotto"}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	second := Assemble([]string{first}, Options{
		TrailingSpace:       false,
		CapitalizeSentences: true,
	})
	require.Equal(t, first, second)
}
