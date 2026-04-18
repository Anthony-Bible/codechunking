package zoekt

import (
	"regexp"
	"strings"

	zoektgrpc "github.com/sourcegraph/zoekt/grpc/protos/zoekt/webserver/v1"
)

// BuildZoektQuery translates an API query string into a Zoekt Q protobuf.
//
// Supported syntax:
//   - ""            → Q_Const{true}  (match all)
//   - "re:pattern"  → Q_Regexp
//   - "file:regex"  → Q_And{Q_Regexp{FileName:true}, Q_Const{true}}
//   - "repo:value"  → Q_And{Q_Repo, Q_Const{true}}
//   - "lang:value"  → Q_And{Q_Language, Q_Const{true}}
//   - "branch:val"  → Q_And{Q_Branch, Q_Const{true}}
//   - "A AND B"     → Q_And{...}
//   - "A OR B"      → Q_Or{...}
//   - "NOT term"    → Q_Not{...}
//   - plain text    → Q_Substring{Content:true, CaseSensitive:false}
func BuildZoektQuery(query string) *zoektgrpc.Q {
	query = strings.TrimSpace(query)
	if query == "" {
		return constTrue()
	}

	// NOT prefix
	if rest, ok := strings.CutPrefix(query, "NOT "); ok {
		child := buildTerm(strings.TrimSpace(rest))
		return &zoektgrpc.Q{
			Query: &zoektgrpc.Q_Not{
				Not: &zoektgrpc.Not{Child: child},
			},
		}
	}

	// AND operator (split on first " AND ")
	if idx := strings.Index(query, " AND "); idx >= 0 {
		left := buildTerm(strings.TrimSpace(query[:idx]))
		rightStr := strings.TrimSpace(query[idx+5:])
		right := buildTerm(rightStr)
		return &zoektgrpc.Q{
			Query: &zoektgrpc.Q_And{
				And: &zoektgrpc.And{Children: []*zoektgrpc.Q{left, right}},
			},
		}
	}

	// OR operator (split on first " OR ")
	if idx := strings.Index(query, " OR "); idx >= 0 {
		left := buildTerm(strings.TrimSpace(query[:idx]))
		rightStr := strings.TrimSpace(query[idx+4:])
		right := buildTerm(rightStr)
		return &zoektgrpc.Q{
			Query: &zoektgrpc.Q_Or{
				Or: &zoektgrpc.Or{Children: []*zoektgrpc.Q{left, right}},
			},
		}
	}

	return buildTerm(query)
}

func buildTerm(term string) *zoektgrpc.Q {
	term = strings.TrimSpace(term)

	if rest, ok := strings.CutPrefix(term, "re:"); ok {
		if _, err := regexp.Compile(rest); err != nil {
			return constTrue()
		}
		return &zoektgrpc.Q{
			Query: &zoektgrpc.Q_Regexp{
				Regexp: &zoektgrpc.Regexp{Regexp: rest},
			},
		}
	}

	if rest, ok := strings.CutPrefix(term, "file:"); ok {
		pattern := globToRegex(rest)
		if _, err := regexp.Compile(pattern); err != nil {
			return constTrue()
		}
		return wrapWithConstTrue(&zoektgrpc.Q{
			Query: &zoektgrpc.Q_Regexp{
				Regexp: &zoektgrpc.Regexp{Regexp: pattern, FileName: true},
			},
		})
	}

	if rest, ok := strings.CutPrefix(term, "repo:"); ok {
		if _, err := regexp.Compile(rest); err != nil {
			return constTrue()
		}
		return wrapWithConstTrue(&zoektgrpc.Q{
			Query: &zoektgrpc.Q_Repo{
				Repo: &zoektgrpc.Repo{Regexp: rest},
			},
		})
	}

	if rest, ok := strings.CutPrefix(term, "lang:"); ok {
		return wrapWithConstTrue(&zoektgrpc.Q{
			Query: &zoektgrpc.Q_Language{
				Language: &zoektgrpc.Language{Language: rest},
			},
		})
	}

	if rest, ok := strings.CutPrefix(term, "branch:"); ok {
		return wrapWithConstTrue(&zoektgrpc.Q{
			Query: &zoektgrpc.Q_Branch{
				Branch: &zoektgrpc.Branch{Pattern: rest},
			},
		})
	}

	return &zoektgrpc.Q{
		Query: &zoektgrpc.Q_Substring{
			Substring: &zoektgrpc.Substring{
				Pattern:       term,
				Content:       true,
				CaseSensitive: false,
			},
		},
	}
}

// globToRegex converts a simple glob pattern (*, ?) to a regex string.
// Literal dots and other regex metacharacters are escaped first.
func globToRegex(glob string) string {
	var b strings.Builder
	for _, ch := range glob {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		case '.', '+', '^', '$', '{', '}', '[', ']', '(', ')', '|', '\\':
			b.WriteByte('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func constTrue() *zoektgrpc.Q {
	return &zoektgrpc.Q{Query: &zoektgrpc.Q_Const{Const: true}}
}

func wrapWithConstTrue(q *zoektgrpc.Q) *zoektgrpc.Q {
	return &zoektgrpc.Q{
		Query: &zoektgrpc.Q_And{And: &zoektgrpc.And{Children: []*zoektgrpc.Q{q, constTrue()}}},
	}
}
