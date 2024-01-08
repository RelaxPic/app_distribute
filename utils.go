package main

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

func ContainsLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// 范围结构体
type Range struct {
	start int64
	end   int64
}

// 解析范围请求头
func parseRange(rangeHeader string, fileSize int64) ([]Range, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, errors.New("Invalid Range header")
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeList := strings.Split(rangeStr, ",")

	ranges := make([]Range, 0, len(rangeList))

	for _, rangeItem := range rangeList {
		rangeParts := strings.Split(rangeItem, "-")
		if len(rangeParts) != 2 {
			return nil, errors.New("Invalid Range header")
		}

		start, err := strconv.ParseInt(rangeParts[0], 10, 64)
		if err != nil || start < 0 {
			return nil, errors.New("Invalid Range header")
		}

		var end int64
		if rangeParts[1] == "" {
			end = fileSize - 1
		} else {
			end, err = strconv.ParseInt(rangeParts[1], 10, 64)
			if err != nil || end < 0 || end >= fileSize {
				return nil, errors.New("Invalid Range header")
			}
		}

		if start > end {
			return nil, errors.New("Invalid Range header")
		}

		ranges = append(ranges, Range{start, end})
	}

	return ranges, nil
}
