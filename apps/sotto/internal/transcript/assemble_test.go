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
