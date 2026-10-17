package metadata

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Image is one piece of artwork a provider has for a title.
type Image struct {
	Path     string  `json:"path"` // TMDB path, e.g. "/abc.jpg"
	Thumb    string  `json:"thumb"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Language string  `json:"language,omitempty"`
	Votes    float64 `json:"votes"`
}

// Images lists TMDB's posters, backdrops and clear logos for a movie (kind "movie") or show
// ("tv"), English and language-free first, most liked first.
func (c *TMDB) Images(ctx context.Context, kind string, id int64) (posters, backdrops, logos []Image, err error) {
	var d struct {
		Posters   []tmdbImage `json:"posters"`
		Backdrops []tmdbImage `json:"backdrops"`
		Logos     []tmdbImage `json:"logos"`
	}
	q := url.Values{"include_image_language": {"en,null"}}
	if err := c.get(ctx, "/"+kind+"/"+strconv.FormatInt(id, 10)+"/images", q, &d); err != nil {
		return nil, nil, nil, err
	}
	conv := func(list []tmdbImage, size string) []Image {
		out := make([]Image, 0, len(list))
		for _, i := range list {
			out = append(out, Image{Path: i.FilePath, Thumb: ImageURL(i.FilePath, size), Width: i.Width, Height: i.Height,
				Language: i.Language, Votes: i.VoteAverage})
		}
		return out
	}
	return conv(d.Posters, "w342"), conv(d.Backdrops, "w780"), conv(pngLogos(d.Logos), "w300"), nil
}

// SeasonImages lists TMDB's posters for one season of a show, English and language-free first.
func (c *TMDB) SeasonImages(ctx context.Context, tvID int64, season int) ([]Image, error) {
	var d struct {
		Posters []tmdbImage `json:"posters"`
	}
	q := url.Values{"include_image_language": {"en,null"}}
	path := "/tv/" + strconv.FormatInt(tvID, 10) + "/season/" + strconv.Itoa(season) + "/images"
	if err := c.get(ctx, path, q, &d); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []Image{}, nil
		}
		return nil, err
	}
	out := make([]Image, 0, len(d.Posters))
	for _, i := range d.Posters {
		out = append(out, Image{Path: i.FilePath, Thumb: ImageURL(i.FilePath, "w342"), Width: i.Width, Height: i.Height,
			Language: i.Language, Votes: i.VoteAverage})
	}
	return out, nil
}

// EpisodeStills lists TMDB's stills for an episode, by its TMDB aired position.
func (c *TMDB) EpisodeStills(ctx context.Context, tvID int64, season, episode int) ([]Image, error) {
	var d struct {
		Stills []tmdbImage `json:"stills"`
	}
	path := "/tv/" + strconv.FormatInt(tvID, 10) + "/season/" + strconv.Itoa(season) + "/episode/" + strconv.Itoa(episode) + "/images"
	if err := c.get(ctx, path, nil, &d); err != nil {
		if errors.Is(err, ErrNotFound) {
			return []Image{}, nil
		}
		return nil, err
	}
	out := make([]Image, 0, len(d.Stills))
	for _, i := range d.Stills {
		out = append(out, Image{Path: i.FilePath, Thumb: ImageURL(i.FilePath, "w300"), Width: i.Width, Height: i.Height,
			Language: i.Language, Votes: i.VoteAverage})
	}
	return out, nil
}

// CollectionImages lists TMDB's posters and backdrops for a collection.
func (c *TMDB) CollectionImages(ctx context.Context, id int64) (posters, backdrops []Image, err error) {
	var d struct {
		Posters   []tmdbImage `json:"posters"`
		Backdrops []tmdbImage `json:"backdrops"`
	}
	q := url.Values{"include_image_language": {"en,null"}}
	if err := c.get(ctx, "/collection/"+strconv.FormatInt(id, 10)+"/images", q, &d); err != nil {
		return nil, nil, err
	}
	conv := func(list []tmdbImage, size string) []Image {
		out := make([]Image, 0, len(list))
		for _, i := range list {
			out = append(out, Image{Path: i.FilePath, Thumb: ImageURL(i.FilePath, size), Width: i.Width, Height: i.Height,
				Language: i.Language, Votes: i.VoteAverage})
		}
		return out
	}
	return conv(d.Posters, "w342"), conv(d.Backdrops, "w780"), nil
}

// pngLogos drops SVG logos, which TMDB doesn't resize.
func pngLogos(list []tmdbImage) []tmdbImage {
	var out []tmdbImage
	for _, i := range list {
		if !strings.HasSuffix(strings.ToLower(i.FilePath), ".svg") {
			out = append(out, i)
		}
	}
	return out
}

// bestLogo picks the logo to show: English before language-free (which is often another
// script), then the most liked. TMDB lists them most liked first.
func bestLogo(list []tmdbImage) string {
	list = pngLogos(list)
	for _, i := range list {
		if i.Language == "en" {
			return i.FilePath
		}
	}
	if len(list) > 0 {
		return list[0].FilePath
	}
	return ""
}

type tmdbImage struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Language    string  `json:"iso_639_1"`
	VoteAverage float64 `json:"vote_average"`
}
