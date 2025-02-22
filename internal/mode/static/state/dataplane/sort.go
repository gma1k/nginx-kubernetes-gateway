package dataplane

import (
	"sort"

	nkgsort "github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/sort"
)

func sortMatchRules(matchRules []MatchRule) {
	// stable sort is used so that the order of matches (as defined in each HTTPRoute rule) is preserved
	// this is important, because the winning match is the first match to win.
	sort.SliceStable(
		matchRules, func(i, j int) bool {
			return higherPriority(matchRules[i], matchRules[j])
		},
	)
}

/*
Returns true if rule1 has a higher priority than rule2.

From the spec:
Precedence must be given to the Rule with the largest number of (Continuing on ties):
- Characters in a matching non-wildcard hostname.
- Characters in a matching hostname.
- Characters in a matching path.
- Method match.
- Header matches.
- Query param matches.

If ties still exist across multiple Routes, matching precedence MUST be determined in order of the following criteria,
continuing on ties:
- The oldest Route based on creation timestamp.
- The Route appearing first in alphabetical order by “{namespace}/{name}”.

If ties still exist within the Route that has been given precedence,
matching precedence MUST be granted to the first matching rule meeting the above criteria.

higherPriority will determine precedence by comparing len(headers), len(query parameters), creation timestamp,
and namespace name. The other criteria are handled by NGINX.
*/
func higherPriority(rule1, rule2 MatchRule) bool {
	// Get the matches from the rules
	match1 := rule1.GetMatch()
	match2 := rule2.GetMatch()

	// Compare if a method exists on one of the matches but not the other.
	// The match with the method specified wins.
	if match1.Method != nil && match2.Method == nil {
		return true
	}
	if match2.Method != nil && match1.Method == nil {
		return false
	}

	// Compare the number of header matches.
	// The match with the largest number of header matches wins.
	l1 := len(match1.Headers)
	l2 := len(match2.Headers)

	if l1 != l2 {
		return l1 > l2
	}
	// If the number of headers is equal then compare the number of query param matches.
	// The match with the most query param matches wins.
	l1 = len(match1.QueryParams)
	l2 = len(match2.QueryParams)

	if l1 != l2 {
		return l1 > l2
	}

	// If still tied, compare the object meta of the two routes.
	return nkgsort.LessObjectMeta(&rule1.Source.ObjectMeta, &rule2.Source.ObjectMeta)
}
