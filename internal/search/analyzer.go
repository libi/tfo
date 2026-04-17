package search

import (
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
)

const (
	AnalyzerName      = "tfo_cjk"
	WordSplitName     = "tfo_word_split"
	BigramUnigramName = "tfo_cjk_bigram"
)

// wordSplitFilter 对非 Ideographic token 按标点/符号切分子 token。
// 例如 "baidu.com" → ["baidu", "com"]，"hello-world" → ["hello", "world"]。
// Ideographic token 不受影响（留给 CJK bigram 处理）。
type wordSplitFilter struct{}

func (f *wordSplitFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	rv := make(analysis.TokenStream, 0, len(input))
	for _, token := range input {
		if token.Type == analysis.Ideographic {
			rv = append(rv, token)
			continue
		}
		// 按非字母非数字字符切分
		sub := splitToken(token)
		rv = append(rv, sub...)
	}
	return rv
}

func splitToken(token *analysis.Token) analysis.TokenStream {
	term := token.Term
	var tokens analysis.TokenStream
	start := 0
	i := 0
	for i < len(term) {
		r, size := utf8.DecodeRune(term[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if i > start {
				tokens = append(tokens, &analysis.Token{
					Term:     term[start:i],
					Start:    token.Start + start,
					End:      token.Start + i,
					Position: token.Position,
					Type:     token.Type,
					KeyWord:  token.KeyWord,
				})
			}
			start = i + size
		}
		i += size
	}
	if start < len(term) {
		tokens = append(tokens, &analysis.Token{
			Term:     term[start:],
			Start:    token.Start + start,
			End:      token.Start + len(term),
			Position: token.Position,
			Type:     token.Type,
			KeyWord:  token.KeyWord,
		})
	}
	if len(tokens) == 0 {
		// 全是标点，保留原 token
		return analysis.TokenStream{token}
	}
	// 修正 position：子 token 递增
	for idx, t := range tokens {
		t.Position = token.Position + idx
	}
	return tokens
}

func wordSplitFilterConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.TokenFilter, error) {
	return &wordSplitFilter{}, nil
}

func init() {
	// 注册自定义 token filter
	_ = registry.RegisterTokenFilter(WordSplitName, wordSplitFilterConstructor)
}
