package judge

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	urlRegex = regexp.MustCompile(`(?i)(https?://|www\.|[a-z0-9-]+\.(com|net|org|io|gg|xyz|ru|cn|tk|me|co)/)`)

	leetspeakReplacer = strings.NewReplacer(
		"0", "o",
		"1", "i",
		"3", "e",
		"4", "a",
		"@", "a",
		"$", "s",
		"5", "s",
		"7", "t",
		"!", "i",
	)

	blockedWords = []string{
		"marico",
		"maricon",
		"puto",
		"puta",
		"hijueputa",
		"hijodeputa",
		"malparido",
		"subnormal",
		"retrasado",
		"nigger",
		"nigga",
		"faggot",
		"retard",
		"kys",
	}

	blockedPhrases = []string{
		"matate",
		"suicidate",
		"kill yourself",
		"go die",
		"me quiero suicidar",
		"quiero suicidarme",
		"suicidarme",
		"cometer suicidio",
		"me voy a suicidar",
		"me quiero matar",
		"quiero matarme",
		"ojala morir",
		"ojala me muera",
		"arma en la cabeza",
		"tiro en la cabeza",
		"quitarme la vida",
		"quitarse la vida",
		"cortarme las venas",
	}
)

type RulesJudge struct{}

func NewRulesJudge() *RulesJudge {
	return &RulesJudge{}
}

func (j *RulesJudge) Name() string {
	return "rules"
}

func (j *RulesJudge) Evaluate(_ context.Context, content Content) (Result, error) {
	combined := strings.TrimSpace(content.Title + " " + content.Body)
	if combined == "" {
		return Result{
			Verdict: Approve,
			Judge:   j.Name(),
			Scores:  map[string]float64{},
		}, nil
	}

	if hasSuspiciousLinks(combined) {
		return Result{
			Verdict: Reject,
			Reason:  "spam_links",
			Judge:   j.Name(),
			Scores:  map[string]float64{"spam_links": 1.0},
		}, nil
	}

	if hasExcessiveRepetition(combined) {
		return Result{
			Verdict: Reject,
			Reason:  "spam_repetitive",
			Judge:   j.Name(),
			Scores:  map[string]float64{"spam_repetitive": 1.0},
		}, nil
	}

	if reason, matched := hasBlockedLanguage(combined); matched {
		return Result{
			Verdict: Reject,
			Reason:  reason,
			Judge:   j.Name(),
			Scores:  map[string]float64{reason: 1.0},
		}, nil
	}

	if isExcessiveShouting(combined) {
		return Result{
			Verdict: Doubt,
			Reason:  "excessive_caps",
			Judge:   j.Name(),
			Scores:  map[string]float64{"excessive_caps": 0.7},
		}, nil
	}

	return Result{
		Verdict: Approve,
		Judge:   j.Name(),
		Scores:  map[string]float64{},
	}, nil
}

func hasSuspiciousLinks(text string) bool {
	return urlRegex.MatchString(text)
}

func hasExcessiveRepetition(text string) bool {
	runes := []rune(strings.ToLower(text))
	if len(runes) >= 12 {
		streak := 1
		for i := 1; i < len(runes); i++ {
			if runes[i] == runes[i-1] && !unicode.IsSpace(runes[i]) {
				streak++
				if streak >= 10 {
					return true
				}
			} else {
				streak = 1
			}
		}
	}

	words := strings.Fields(strings.ToLower(text))
	if len(words) >= 12 {
		unique := make(map[string]struct{}, len(words))
		for _, w := range words {
			unique[w] = struct{}{}
		}
		if float64(len(unique))/float64(len(words)) < 0.2 {
			return true
		}
	}

	return false
}

func hasBlockedLanguage(text string) (string, bool) {
	normalized := normalizeText(text)

	for _, phrase := range blockedPhrases {
		if strings.Contains(normalized, phrase) {
			return "self_harm_or_threat", true
		}
	}

	tokens := tokenizeWords(normalized)
	for _, token := range tokens {
		if slices.Contains(blockedWords, token) {
			return "hate_or_harassment", true
		}
	}

	return "", false
}

func isExcessiveShouting(text string) bool {
	var letters, upper int
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if letters < 40 {
		return false
	}
	return float64(upper)/float64(letters) > 0.8
}

func normalizeText(text string) string {
	lower := strings.ToLower(text)
	replaced := leetspeakReplacer.Replace(lower)

	accentReplacer := strings.NewReplacer(
		"á", "a",
		"é", "e",
		"í", "i",
		"ó", "o",
		"ú", "u",
		"ü", "u",
	)
	replaced = accentReplacer.Replace(replaced)

	var collapsed strings.Builder
	var prev rune
	var count int
	for _, r := range replaced {
		if r == prev {
			count++
			if count >= 2 {
				continue
			}
		} else {
			prev = r
			count = 0
		}
		collapsed.WriteRune(r)
	}

	return collapsed.String()
}

func tokenizeWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}
