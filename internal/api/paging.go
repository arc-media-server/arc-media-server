package api

import (
	"fmt"
	"strconv"
)

func pageBounds(rawLimit, rawOffset string, defaultLimit, maxLimit int) (int, int, error) {
	limit, offset := defaultLimit, 0
	var err error
	if rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > maxLimit {
			return 0, 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
		}
	}
	if rawOffset != "" {
		offset, err = strconv.Atoi(rawOffset)
		if err != nil || offset < 0 || offset > 10000000 {
			return 0, 0, fmt.Errorf("offset must be between 0 and 10000000")
		}
	}
	return limit, offset, nil
}
