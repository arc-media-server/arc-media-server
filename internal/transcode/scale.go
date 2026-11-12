package transcode

import "fmt"

// FitScaleFilter caps both dimensions of a quality's 16:9 box, keeps aspect ratio, and never
// upscales. Even dimensions work with 4:2:0 encoders. Source metadata isn't needed for safety.
func FitScaleFilter(height int) string {
	return fmt.Sprintf("scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", height*16/9, height)
}
