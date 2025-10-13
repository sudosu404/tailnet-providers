package rules

import (
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	gperr "github.com/yusing/goutils/errs"
)

type (
	Matcher     func(string) bool
	MatcherType string
)

const (
	MatcherTypeString MatcherType = "string"
	MatcherTypeGlob   MatcherType = "glob"
	MatcherTypeRegex  MatcherType = "regex"
)

func unquoteExpr(s string) (string, gperr.Error) {
	if s == "" {
		return "", nil
	}
	switch s[0] {
	case '"', '\'', '`':
		if s[0] != s[len(s)-1] {
			return "", ErrUnterminatedQuotes
		}
		return s[1 : len(s)-1], nil
	default:
		return s, nil
	}
}

func ExtractExpr(s string) (matcherType MatcherType, expr string, err gperr.Error) {
	idx := strings.IndexByte(s, '(')
	if idx == -1 {
		return MatcherTypeString, s, nil
	}
	idxEnd := strings.LastIndexByte(s, ')')
	if idxEnd == -1 {
		return "", "", ErrUnterminatedBrackets
	}

	expr, err = unquoteExpr(s[idx+1 : idxEnd])
	if err != nil {
		return "", "", err
	}
	matcherType = MatcherType(strings.ToLower(s[:idx]))

	switch matcherType {
	case MatcherTypeGlob, MatcherTypeRegex, MatcherTypeString:
		return
	default:
		return "", "", ErrInvalidArguments.Withf("invalid matcher type: %s", matcherType)
	}
}

func ParseMatcher(expr string) (Matcher, gperr.Error) {
	negate := false
	if strings.HasPrefix(expr, "!") {
		negate = true
		expr = expr[1:]
	}

	t, expr, err := ExtractExpr(expr)
	if err != nil {
		return nil, err
	}

	switch t {
	case MatcherTypeString:
		return StringMatcher(expr, negate)
	case MatcherTypeGlob:
		return GlobMatcher(expr, negate)
	case MatcherTypeRegex:
		return RegexMatcher(expr, negate)
	}
	// won't reach here
	return nil, ErrInvalidArguments.Withf("invalid matcher type: %s", t)
}

func StringMatcher(s string, negate bool) (Matcher, gperr.Error) {
	if negate {
		return func(s2 string) bool {
			return s != s2
		}, nil
	}
	return func(s2 string) bool {
		return s == s2
	}, nil
}

func GlobMatcher(expr string, negate bool) (Matcher, gperr.Error) {
	g, err := glob.Compile(expr)
	if err != nil {
		return nil, ErrInvalidArguments.With(err)
	}
	if negate {
		return func(s string) bool {
			return !g.Match(s)
		}, nil
	}
	return g.Match, nil
}

func RegexMatcher(expr string, negate bool) (Matcher, gperr.Error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, ErrInvalidArguments.With(err)
	}
	if negate {
		return func(s string) bool {
			return !re.MatchString(s)
		}, nil
	}
	return re.MatchString, nil
}
