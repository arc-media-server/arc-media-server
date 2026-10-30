package store

import "strings"

// Media filters must match the same present file. Audio criteria share one audio stream,
// video criteria share one video stream. Series match only an accessible child episode.
func mediaFilter(q ItemQuery) (string, []any) {
	if q.VideoCodec == "" && q.AudioCodec == "" && q.AudioChannels == 0 && q.DynamicRange == "" {
		return "", nil
	}
	var args []any
	cond := []string{"m.missing_since IS NULL"}
	var video, audio []string
	if q.VideoCodec != "" {
		video = append(video, "st.codec = ? COLLATE NOCASE")
		args = append(args, q.VideoCodec)
	}
	if q.DynamicRange != "" {
		video = append(video, "("+dynamicRangeSQL+" = ?)")
		args = append(args, q.DynamicRange)
	}
	if len(video) > 0 {
		cond = append(cond, "EXISTS (SELECT 1 FROM media_streams st WHERE st.file_id=m.id AND st.kind='video' AND "+strings.Join(video, " AND ")+")")
	}
	if q.AudioCodec != "" {
		audio = append(audio, "st.codec = ? COLLATE NOCASE")
		args = append(args, q.AudioCodec)
	}
	if q.AudioChannels > 0 {
		audio = append(audio, "st.channels = ?")
		args = append(args, q.AudioChannels)
	}
	if len(audio) > 0 {
		cond = append(cond, "EXISTS (SELECT 1 FROM media_streams st WHERE st.file_id=m.id AND st.kind='audio' AND "+strings.Join(audio, " AND ")+")")
	}
	child, childArgs := q.Access.Conds("e")
	args = append(childArgs, args...)
	identity := "(e.id=items.id OR (items.kind='series' AND e.parent_id=items.id))"
	if len(child) > 0 {
		identity += " AND " + strings.Join(child, " AND ")
	}
	return "EXISTS (SELECT 1 FROM items e JOIN item_files f ON f.item_id=e.id JOIN media_files m ON m.id=f.file_id WHERE " + identity + " AND " + strings.Join(cond, " AND ") + ")", args
}

// A source classification, not a playback/output guarantee. Unknown is never silently SDR.
const dynamicRangeSQL = `CASE WHEN st.dolby_vision=1 THEN 'dolby_vision'
 WHEN st.hdr10_plus=1 THEN 'hdr10_plus' WHEN st.color_transfer='arib-std-b67' THEN 'hlg'
 WHEN st.color_transfer='smpte2084' THEN 'hdr10'
 WHEN st.bit_depth>0 OR st.pix_fmt<>'' OR st.color_transfer<>'' THEN 'sdr' ELSE 'unknown' END`
