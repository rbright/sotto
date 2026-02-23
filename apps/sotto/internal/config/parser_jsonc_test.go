package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeJSONCRemovesCommentsAndTrailingCommas(t *testing.T) {
	input := `
{
  // line comment
  "items": [
    "one", /* block comment */
    "two",
  ],
  "nested": {
    "enabled": true,
  },
}
`

	normalized, err := normalizeJSONC(input)
	require.NoError(t, err)
	require.NotContains(t, normalized, "//")
	require.NotContains(t, normalized, "/*")
	require.NotContains(t, normalized, ",]")
	require.NotContains(t, normalized, ",}")
}

func TestNormalizeJSONCRetainsCommentLikeTextInsideStrings(t *testing.T) {
	input := `{"value":"contains // and /* comment-like */ text",}`
	normalized, err := normalizeJSONC(input)
	require.NoError(t, err)
	require.Contains(t, normalized, "// and /* comment-like */")
}

func TestNormalizeJSONCUnterminatedBlockCommentFails(t *testing.T) {
	_, err := normalizeJSONC("{ /* unterminated ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unterminated block comment")
}

func TestEnsureSingleJSONValueRejectsExtraPayload(t *testing.T) {
	decoder := json.NewDecoder(strings.NewReader(`{"one":1}{"two":2}`))
	var payload map[string]any
	require.NoError(t, decoder.Decode(&payload))

	err := ensureSingleJSONValue(decoder)
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple JSON values")
}

func TestOffsetToLineCol(t *testing.T) {
	content := "line1\nline2\nline3"
	line, col := offsetToLineCol(content, 1)
	require.Equal(t, 1, line)
	require.Equal(t, 1, col)

	line, col = offsetToLineCol(content, 8) // line2, col2
	require.Equal(t, 2, line)
	require.Equal(t, 2, col)

	line, col = offsetToLineCol(content, 999)
	require.Equal(t, 3, line)
	require.Equal(t, 5, col)
}

func TestJSONCStringListUnmarshal(t *testing.T) {
	var list jsoncStringList
	require.NoError(t, list.UnmarshalJSON([]byte(`["a","b"]`)))
	require.Equal(t, []string{"a", "b"}, []string(list))

	require.NoError(t, list.UnmarshalJSON([]byte(`"a, b, , c"`)))
	require.Equal(t, []string{"a", "b", "c"}, []string(list))

	err := list.UnmarshalJSON([]byte(`123`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string array")
}

func TestParseJSONCRejectsInvalidCommandArgv(t *testing.T) {
	_, _, err := parseJSONC(`{"clipboard_cmd":"unterminated ' quote"}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid clipboard_cmd")

	_, _, err = parseJSONC(`{"paste_cmd":"unterminated ' quote"}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid paste_cmd")
}

func TestParseJSONCVocabRejectsEmptySetName(t *testing.T) {
	_, _, err := parseJSONC(`{"vocab":{"sets":{" ":{"phrases":["x"]}}}}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty set name")
}

func TestParseJSONCTrimsIndicatorAndPasteFields(t *testing.T) {
	cfg, _, err := parseJSONC(`{
  "paste": {"shortcut": "  CTRL,V  "},
  "indicator": {
    "backend": " desktop ",
    "desktop_app_name": "  sotto-indicator  "
  }
}`, Default())
	require.NoError(t, err)
	require.Equal(t, "CTRL,V", cfg.Paste.Shortcut)
	require.Equal(t, "desktop", cfg.Indicator.Backend)
	require.Equal(t, "sotto-indicator", cfg.Indicator.DesktopAppName)
}

func TestParseJSONCRejectsMultipleTopLevelValues(t *testing.T) {
	_, _, err := parseJSONC(`{"paste":{"enable":false}}{"paste":{"enable":true}}`, Default())
	require.Error(t, err)
	require.True(
		t,
		strings.Contains(err.Error(), "multiple JSON values") || strings.Contains(err.Error(), "unknown field"),
		"unexpected error: %v",
		err,
	)
}

func TestParseJSONCTypeErrorIncludesLocation(t *testing.T) {
	_, _, err := parseJSONC(`{
  "riva": {"grpc": 123}
}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "line")
	require.Contains(t, err.Error(), "column")
}

func TestParseJSONCVocabGlobalSupportsCommaString(t *testing.T) {
	cfg, _, err := parseJSONC(`{
  "vocab": {
    "global": "one, two, , three",
    "sets": {
      "one": {"phrases": ["one"]},
      "two": {"phrases": ["two"]},
      "three": {"phrases": ["three"]}
    }
  }
}`, Default())
	require.NoError(t, err)
	require.Equal(t, []string{"one", "two", "three"}, cfg.Vocab.GlobalSets)
}
