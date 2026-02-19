package transcript

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssembleNormalizesWhitespaceAndTrailingSpace(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{" hello", "world  ", "\nfrom", "sotto"}, true)
	require.Equal(t, "hello world from sotto ", got)
}

func TestAssembleWithoutTrailingSpace(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"hello", "world"}, false)
	require.Equal(t, "hello world", got)
}

func TestAssembleEmptyInput(t *testing.T) {
	t.Parallel()

	require.Empty(t, Assemble(nil, true))
}

func TestAssembleSkipsWhitespaceOnlySegments(t *testing.T) {
	t.Parallel()

	got := Assemble([]string{"  ", "\n\t", "hello"}, false)
	require.Equal(t, "hello", got)
}

func TestAssembleIdempotentForNormalizedOutput(t *testing.T) {
	t.Parallel()

	first := Assemble([]string{"hello", "world"}, false)
	second := Assemble([]string{first}, false)
	require.Equal(t, first, second)
}
