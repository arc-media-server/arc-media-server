package store

// DolbyVisionBaseFormat reports only a declared backwards-compatible base, not output-device
// capability. Unknown/zero compatibility remains unsafe. Legacy profile 7 has an HDR10 base.
func (s *Stream) DolbyVisionBaseFormat() (string, bool) {
	if !s.DolbyVision {
		return "", false
	}
	if s.DVBLCompatibilityID == nil {
		if s.DolbyVisionProfile == 7 {
			return "hdr10", true
		}
		return "", false
	}
	if s.DolbyVisionProfile != 7 && s.DolbyVisionProfile != 8 {
		return "", false
	}
	if s.DolbyVisionProfile == 7 {
		return "hdr10", *s.DVBLCompatibilityID == 6
	}
	switch *s.DVBLCompatibilityID {
	case 1:
		return "hdr10", true
	case 2:
		return "", true
	case 4:
		return "hlg", true
	}
	return "", false
}
