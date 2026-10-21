package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// episodeRange is an inclusive range of episode numbers. A single episode is
// represented as a range whose first and last values are equal.
type episodeRange struct {
	first int
	last  int
}

// parseEpisodeSelection parses a comma-separated list of episode numbers and
// ranges, such as "1-12,15,20-24". An empty spec means "no filter" and returns
// nil. Episode numbers are per season, so 0 is allowed for specials.
func parseEpisodeSelection(spec string) ([]episodeRange, error) {
	if spec == "" {
		return nil, nil
	}

	var ranges []episodeRange
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty episode number in %q", spec)
		}

		firstText, lastText, isRange := strings.Cut(part, "-")
		first, err := parseEpisodeNumber(firstText)
		if err != nil {
			return nil, err
		}

		last := first
		if isRange {
			last, err = parseEpisodeNumber(lastText)
			if err != nil {
				return nil, err
			}
			if last < first {
				return nil, fmt.Errorf("invalid episode range %q: end is before start", part)
			}
		}

		ranges = append(ranges, episodeRange{first: first, last: last})
	}
	return ranges, nil
}

// parseEpisodeNumber parses one non-negative episode number.
func parseEpisodeNumber(text string) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || number < 0 {
		return 0, fmt.Errorf("invalid episode number %q", text)
	}
	return number, nil
}

// contains reports whether n falls inside the range.
func (r episodeRange) contains(n int) bool {
	return n >= r.first && n <= r.last
}

// formatEpisodeNumbers compresses the episode numbers in episodes into a
// comma-separated list of ranges, e.g. "1-3,5,7-8". It is used to show which
// numbers a season actually has when a selection matches nothing.
func formatEpisodeNumbers(episodes []SeasonEpisode) string {
	numbers := make([]int, 0, len(episodes))
	for _, episode := range episodes {
		numbers = append(numbers, episode.EpisodeNumber)
	}
	slices.Sort(numbers)
	numbers = slices.Compact(numbers)

	var parts []string
	for i := 0; i < len(numbers); {
		j := i
		for j+1 < len(numbers) && numbers[j+1] == numbers[j]+1 {
			j++
		}
		if i == j {
			parts = append(parts, strconv.Itoa(numbers[i]))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", numbers[i], numbers[j]))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// filterEpisodes returns the episodes whose numbers match any selected range,
// preserving the API's order. With no ranges every episode is returned, so an
// unset -episodes flag leaves existing behavior unchanged.
func filterEpisodes(episodes []SeasonEpisode, ranges []episodeRange) []SeasonEpisode {
	if len(ranges) == 0 {
		return episodes
	}

	selected := make([]SeasonEpisode, 0, len(episodes))
	for _, episode := range episodes {
		for _, r := range ranges {
			if r.contains(episode.EpisodeNumber) {
				selected = append(selected, episode)
				break
			}
		}
	}
	return selected
}
