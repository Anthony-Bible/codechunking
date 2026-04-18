package zoekt

import (
	"testing"

	zoektgrpc "github.com/sourcegraph/zoekt/grpc/protos/zoekt/webserver/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildZoektQuery defines the contract for the BuildZoektQuery function.
// BuildZoektQuery translates an API query string into a Zoekt Q protobuf.
//
// Contract: BuildZoektQuery(query string) *zoektgrpc.Q.
func TestBuildZoektQuery(t *testing.T) {
	t.Run("Plain_Text_Query_Returns_Substring_Q", func(t *testing.T) {
		q := BuildZoektQuery("authenticate user")

		require.NotNil(t, q, "result must not be nil")
		sub, ok := q.GetQuery().(*zoektgrpc.Q_Substring)
		require.True(t, ok, "plain text query must produce Q_Substring, got %T", q.GetQuery())
		assert.Equal(t, "authenticate user", sub.Substring.GetPattern(), "pattern must match input")
		assert.True(t, sub.Substring.GetContent(), "content flag must be true")
		assert.False(t, sub.Substring.GetCaseSensitive(), "search must be case-insensitive by default")
	})

	t.Run("Re_Prefix_Returns_Regexp_Q", func(t *testing.T) {
		q := BuildZoektQuery("re:func\\s+\\w+")

		require.NotNil(t, q)
		re, ok := q.GetQuery().(*zoektgrpc.Q_Regexp)
		require.True(t, ok, "re: prefix must produce Q_Regexp, got %T", q.GetQuery())
		assert.Equal(t, "func\\s+\\w+", re.Regexp.GetRegexp(), "regexp pattern must be the part after 're:'")
	})

	t.Run("File_Prefix_Returns_Q_And_With_FileName_Filter", func(t *testing.T) {
		// Use valid regex syntax (not glob *.go) to exercise the happy path.
		q := BuildZoektQuery(`file:.*\.go$`)

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "file: prefix must produce Q_And composite, got %T", q.GetQuery())
		require.NotEmpty(t, and.And.GetChildren(), "Q_And must have children")

		// One of the children must be a Q_Regexp with FileName=true
		hasFileName := false
		for _, child := range and.And.GetChildren() {
			if re, isRegexp := child.GetQuery().(*zoektgrpc.Q_Regexp); isRegexp {
				if re.Regexp.GetFileName() {
					hasFileName = true
				}
			}
		}
		assert.True(t, hasFileName, "Q_And must contain a Q_Regexp child with FileName=true")
	})

	t.Run("Repo_Prefix_Returns_Q_And_With_Repo_Filter", func(t *testing.T) {
		q := BuildZoektQuery("repo:myrepo")

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "repo: prefix must produce Q_And composite, got %T", q.GetQuery())

		hasRepo := false
		for _, child := range and.And.GetChildren() {
			if r, isRepo := child.GetQuery().(*zoektgrpc.Q_Repo); isRepo {
				hasRepo = true
				assert.Equal(t, "myrepo", r.Repo.GetRegexp(), "repo filter regexp must match the value after 'repo:'")
			}
		}
		assert.True(t, hasRepo, "Q_And must contain a Q_Repo child")
	})

	t.Run("Lang_Prefix_Returns_Q_And_With_Language_Filter", func(t *testing.T) {
		q := BuildZoektQuery("lang:go")

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "lang: prefix must produce Q_And composite, got %T", q.GetQuery())

		hasLang := false
		for _, child := range and.And.GetChildren() {
			if l, isLang := child.GetQuery().(*zoektgrpc.Q_Language); isLang {
				hasLang = true
				assert.Equal(t, "go", l.Language.GetLanguage(), "language must match value after 'lang:'")
			}
		}
		assert.True(t, hasLang, "Q_And must contain a Q_Language child")
	})

	t.Run("Branch_Prefix_Returns_Q_And_With_Branch_Filter", func(t *testing.T) {
		q := BuildZoektQuery("branch:main")

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "branch: prefix must produce Q_And composite, got %T", q.GetQuery())

		hasBranch := false
		for _, child := range and.And.GetChildren() {
			if b, isBranch := child.GetQuery().(*zoektgrpc.Q_Branch); isBranch {
				hasBranch = true
				assert.Equal(t, "main", b.Branch.GetPattern(), "branch pattern must match value after 'branch:'")
			}
		}
		assert.True(t, hasBranch, "Q_And must contain a Q_Branch child")
	})

	t.Run("AND_Operator_Returns_Q_And_Composite", func(t *testing.T) {
		q := BuildZoektQuery("http AND server")

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "AND operator must produce Q_And, got %T", q.GetQuery())
		assert.GreaterOrEqual(t, len(and.And.GetChildren()), 2, "Q_And must have at least 2 children for two terms")
	})

	t.Run("OR_Operator_Returns_Q_Or_Composite", func(t *testing.T) {
		q := BuildZoektQuery("http OR server")

		require.NotNil(t, q)
		or, ok := q.GetQuery().(*zoektgrpc.Q_Or)
		require.True(t, ok, "OR operator must produce Q_Or, got %T", q.GetQuery())
		assert.GreaterOrEqual(t, len(or.Or.GetChildren()), 2, "Q_Or must have at least 2 children for two terms")
	})

	t.Run("NOT_Prefix_Returns_Q_Not_Composite", func(t *testing.T) {
		q := BuildZoektQuery("NOT deprecated")

		require.NotNil(t, q)
		not, ok := q.GetQuery().(*zoektgrpc.Q_Not)
		require.True(t, ok, "NOT operator must produce Q_Not, got %T", q.GetQuery())
		require.NotNil(t, not.Not.GetChild(), "Q_Not must wrap a child query")
	})

	t.Run("Mixed_Func_AND_File_Returns_Q_And_With_Both", func(t *testing.T) {
		// Use a valid regex for file: (not glob syntax like *.go)
		q := BuildZoektQuery(`func AND file:.*\.go$`)

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "mixed AND with file: filter must produce Q_And, got %T", q.GetQuery())

		hasFileName := false
		hasSubstring := false
		for _, child := range and.And.GetChildren() {
			switch v := child.GetQuery().(type) {
			case *zoektgrpc.Q_Regexp:
				if v.Regexp.GetFileName() {
					hasFileName = true
				}
			case *zoektgrpc.Q_Substring:
				hasSubstring = true
			case *zoektgrpc.Q_And:
				// The file: term itself is wrapped in Q_And; check its children for Q_Regexp+FileName
				for _, grandchild := range v.And.GetChildren() {
					if re, isRegexp := grandchild.GetQuery().(*zoektgrpc.Q_Regexp); isRegexp && re.Regexp.GetFileName() {
						hasFileName = true
					}
				}
			}
		}
		assert.True(t, hasFileName, `Q_And must contain Q_Regexp with FileName=true child for 'file:.*\.go$'`)
		assert.True(t, hasSubstring, "Q_And must contain Q_Substring child for 'func'")
	})

	t.Run("Empty_Query_Returns_Q_Const_True", func(t *testing.T) {
		q := BuildZoektQuery("")

		require.NotNil(t, q, "empty query must still return a non-nil Q")
		c, ok := q.GetQuery().(*zoektgrpc.Q_Const)
		require.True(t, ok, "empty query must produce Q_Const, got %T", q.GetQuery())
		assert.True(t, c.Const, "Q_Const must be true (match all) for empty query")
	})

	t.Run("File_Glob_Pattern_Translated_To_Regex", func(t *testing.T) {
		// *.go is glob syntax — it is translated to .*\.go and matched as a filename regexp.
		q := BuildZoektQuery("file:*.go")

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "glob file: pattern must produce Q_And, got %T", q.GetQuery())

		hasFileName := false
		for _, child := range and.And.GetChildren() {
			if re, isRegexp := child.GetQuery().(*zoektgrpc.Q_Regexp); isRegexp && re.Regexp.GetFileName() {
				hasFileName = true
				assert.Equal(t, `.*\.go`, re.Regexp.GetRegexp(), "glob *.go must translate to .*\\.go regex")
			}
		}
		assert.True(t, hasFileName, "Q_And must contain a FileName regexp child")
	})

	t.Run("File_Valid_Regex_Returns_Q_And_With_FileName_Regexp", func(t *testing.T) {
		q := BuildZoektQuery(`file:.*\.go$`)

		require.NotNil(t, q)
		and, ok := q.GetQuery().(*zoektgrpc.Q_And)
		require.True(t, ok, "valid file: regex must produce Q_And, got %T", q.GetQuery())

		hasFileName := false
		for _, child := range and.And.GetChildren() {
			if re, isRegexp := child.GetQuery().(*zoektgrpc.Q_Regexp); isRegexp && re.Regexp.GetFileName() {
				hasFileName = true
			}
		}
		assert.True(t, hasFileName, "Q_And must contain Q_Regexp with FileName=true for a valid file: pattern")
	})
}
