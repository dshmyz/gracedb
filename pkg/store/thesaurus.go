package store

import "strings"

// synonymGroups defines groups of interchangeable terms.
// Keys are terms as produced by the gse segmenter.
var synonymGroups = map[string]string{
	// AI/Technology
	"ai":           "artificial intelligence",
	"人工智能":      "artificial intelligence",
	"ml":           "machine learning",
	"机器":         "machine learning",
	"学习":         "machine learning",
	"dl":           "deep learning",
	"深度":         "deep learning",
	"nlp":          "natural language processing",
	"自然":         "natural language processing",
	"语言":         "natural language processing",
	"llm":          "large language model",
	"模型":         "large language model",

	// Programming
	"golang":       "go",
	"js":           "javascript",
	"py":           "python",
	"代码":         "code",
	"程序":         "code",

	// Common aliases
	"db":           "database",
	"数据库":        "database",
	"api":          "api interface",
	"接口":         "api interface",
	"向量":         "vector embedding",
	"embed":        "vector embedding",
	"检索":         "search",
	"搜索":         "search",
}

// canonicalTerms maps canonical phrases back to their source terms.
var canonicalTerms = make(map[string][]string)

func init() {
	for syn, canon := range synonymGroups {
		canonicalTerms[canon] = append(canonicalTerms[canon], syn)
	}
}

// expandSynonyms returns all synonyms for a term, including the term itself.
func expandSynonyms(term string) []string {
	lower := strings.ToLower(term)
	results := []string{lower}

	if canon, ok := synonymGroups[lower]; ok {
		results = append(results, canon)
	}

	for syn, canon := range synonymGroups {
		if canon == lower && syn != lower {
			results = append(results, syn)
		}
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(results))
	for _, r := range results {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// applySynonymIndexing returns all terms to index for a given token.
func applySynonymIndexing(token string) []string {
	return expandSynonyms(token)
}
