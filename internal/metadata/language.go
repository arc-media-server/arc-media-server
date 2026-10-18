package metadata

import (
	"context"
	"regexp"
	"strings"
)

type languageKey struct{}

// WithLanguage makes TMDB requests made with ctx ask for titles and descriptions in a language
// (e.g. "fr-FR", or "en-GB" for British titles). "" means TMDB's default, American English.
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, languageKey{}, lang)
}

func languageOf(ctx context.Context) string {
	lang, _ := ctx.Value(languageKey{}).(string)
	return lang
}

// needsEnglish reports whether ctx asks for a language other than English, so missing
// translations should be filled in from English.
func needsEnglish(ctx context.Context) bool {
	lang := languageOf(ctx)
	return lang != "" && !strings.HasPrefix(lang, "en")
}

// english is ctx asking for English, for filling in what isn't translated.
func english(ctx context.Context) context.Context { return WithLanguage(ctx, "en-US") }

// untranslatedTitle reports whether a title came back untranslated: TMDB then gives the original
// title, which in a language you don't read is no help (an English one is).
func untranslatedTitle(ctx context.Context, title, original, originalLanguage string) bool {
	lang := languageOf(ctx)
	return title == original && originalLanguage != "" && !strings.HasPrefix(lang, originalLanguage)
}

// genericEpisodeName matches TMDB's stand-in names for untranslated episodes ("Episode 3",
// "Épisode 3", "Folge 3", "第3話").
var genericEpisodeName = regexp.MustCompile(`^\D{0,12}\d+\D{0,2}$`)

// tvdbLanguage is the library's language as TheTVDB names it ("fra"), or "eng".
func tvdbLanguage(ctx context.Context) string {
	lang := languageOf(ctx)
	if lang == "pt-BR" {
		return "pt" // TheTVDB's Brazilian Portuguese
	}
	if l, ok := tvdbLanguages[strings.SplitN(lang, "-", 2)[0]]; ok {
		return l
	}
	return "eng"
}

var tvdbLanguages = map[string]string{
	"ar": "ara", "cs": "ces", "da": "dan", "de": "deu", "el": "ell", "es": "spa", "fi": "fin", "fr": "fra",
	"he": "heb", "hi": "hin", "hu": "hun", "id": "ind", "it": "ita", "ja": "jpn", "ko": "kor", "nb": "nor",
	"nl": "nld", "pl": "pol", "pt": "por", "ro": "ron", "ru": "rus", "sv": "swe", "th": "tha", "tr": "tur",
	"uk": "ukr", "vi": "vie", "zh": "zho",
}
