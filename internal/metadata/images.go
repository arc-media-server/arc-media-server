package metadata

import (
	"context"
	"net/url"
	"strconv"
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

// Images lists TMDB's posters and backdrops for a movie (kind "movie") or show ("tv"), English and
// language-free first, most liked first.
func (c *TMDB) Images(ctx context.Context, kind string, id int64) (posters, backdrops []Image, err error) {
	var d struct {
		Posters   []tmdbImage `json:"posters"`
		Backdrops []tmdbImage `json:"backdrops"`
	}
	q := url.Values{"include_image_language": {"en,null"}}
	if err := c.get(ctx, "/"+kind+"/"+strconv.FormatInt(id, 10)+"/images", q, &d); err != nil {
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

type tmdbImage struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Language    string  `json:"iso_639_1"`
	VoteAverage float64 `json:"vote_average"`
}
