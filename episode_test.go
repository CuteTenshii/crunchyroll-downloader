package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParsePlaybackAPIErrorAcceptsNumericAndString4294(t *testing.T) {
	for _, raw := range []string{"4294", `"4294"`} {
		err := parsePlaybackAPIError("GY19Q77GR", []byte(raw))
		if err.EpisodeID != "GY19Q77GR" || err.Code != playback4294Code {
			t.Fatalf("parsePlaybackAPIError(%s) = %#v", raw, err)
		}
		if strings.Contains(err.Error(), raw) && strings.Contains(raw, `"`) {
			t.Fatalf("typed error exposed raw JSON syntax: %q", err.Error())
		}
	}
}

func TestParsePlaybackAPIErrorNeverFormatsRawObject(t *testing.T) {
	raw := []byte(`{"code":5001,"message":"denied https://api.example/token/sentinel?secret=value","token":"body-secret"}`)
	err := parsePlaybackAPIError("episode", raw)
	if err.Code != "5001" || strings.Contains(err.Error(), "sentinel") || strings.Contains(err.Error(), "value") || strings.Contains(err.Error(), "body-secret") {
		t.Fatalf("unsafe typed playback error: %q", err.Error())
	}
}

func TestOpenPlaybackWithRetryNon4294DoesNotRetry(t *testing.T) {
	calls := 0
	defer func() {
		recovered := recover()
		var playbackErr *PlaybackAPIError
		if !errors.As(asError(recovered), &playbackErr) || playbackErr.Code != "5001" {
			t.Fatalf("recovered = %#v", recovered)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	}()
	openPlaybackWithRetry("episode", func(string) Episode {
		calls++
		panic(&PlaybackAPIError{EpisodeID: "episode", Code: "5001"})
	}, 3, time.Second, func(time.Duration) { t.Fatal("unexpected sleep") })
}

func TestOpenPlaybackWithRetryThenSucceeds(t *testing.T) {
	calls := 0
	var delays []time.Duration
	episode := openPlaybackWithRetry("episode", func(string) Episode {
		calls++
		if calls == 1 {
			panic(&PlaybackAPIError{EpisodeID: "episode", Code: playback4294Code})
		}
		return Episode{Token: "stream"}
	}, 2, 3*time.Second, func(delay time.Duration) { delays = append(delays, delay) })
	if calls != 2 || episode.Token != "stream" || !reflect.DeepEqual(delays, []time.Duration{3 * time.Second}) {
		t.Fatalf("calls=%d episode=%#v delays=%v", calls, episode, delays)
	}
}

func TestOpenPlaybackWithRetryExhaustsBoundedAttemptsAndBackoff(t *testing.T) {
	calls := 0
	var delays []time.Duration
	defer func() {
		recovered := recover()
		var playbackErr *PlaybackAPIError
		if !errors.As(asError(recovered), &playbackErr) || playbackErr.Code != playback4294Code {
			t.Fatalf("recovered = %#v", recovered)
		}
		if calls != 4 {
			t.Fatalf("calls = %d, want 4", calls)
		}
		want := []time.Duration{20 * time.Second, 40 * time.Second, time.Minute}
		if !reflect.DeepEqual(delays, want) {
			t.Fatalf("delays = %v, want %v", delays, want)
		}
	}()
	openPlaybackWithRetry("episode", func(string) Episode {
		calls++
		panic(&PlaybackAPIError{EpisodeID: "episode", Code: playback4294Code})
	}, 3, 20*time.Second, func(delay time.Duration) { delays = append(delays, delay) })
}

func asError(value any) error {
	err, _ := value.(error)
	return err
}
