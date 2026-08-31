package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	widevine "github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

const (
	playReadySeconds  = 4.0
	playReadySegments = 3
	playingFileName   = "playing.mp4"
	audioFileName     = "audio.mp4"
	initFileName      = "init.mp4"
	playVideoWorkers  = 6
	playAudioWorkers  = 4
)

// PlayProgress is emitted as the progressive buffer grows. Ready is set once
// when BufferEndSec >= 4s or 3 contiguous media segments are present.
type PlayProgress struct {
	BufferEndSec float64
	DurationSec  float64
	Ready        bool
	PlayingPath  string
	Err          error
}

// PlaySession is a forced-quality (never ABR) decrypt-while-watching session.
// playing.mp4 is built by writing ftyp+moov from the first decrypted fragment
// and appending subsequent moof+mdat CMAF samples. SeekTarget retargets the
// worker queue; BufferEndSec stays the contiguous prefix from 0.
type PlaySession struct {
	EpisodeID    string
	VideoQuality string
	AudioQuality string
	BufferEndSec float64 // contiguous from 0
	Dir          string  // temp session dir

	mu              sync.Mutex
	queue           *segmentQueue
	audioQueue      *segmentQueue
	segDurations    []float64
	audioDurations  []float64
	durationSec     float64
	have            []bool
	audioHave       []bool
	pending         [][]byte
	audioPending    [][]byte
	videoNums       []int64
	audioNums       []int64
	contiguous      int
	audioContiguous int
	playingFile     string
	audioFile       string
	videoInit       []byte
	audioInit       []byte
	keys            []*widevine.Key
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	contentID       string
	streamToken     string
	readyEmitted    bool
	closed          bool
	emit            func(PlayProgress)
	readyCh         chan struct{}
}

// segmentQueue hands out segment indexes. Retarget makes that index the next
// job; subsequent Next values fill forward to the end, then wrap to fill holes
// from 0. Never used for ABR / representation switching.
type segmentQueue struct {
	mu        sync.Mutex
	n         int
	taken     []bool
	next      int
	remaining int
	stopped   bool
}

func newSegmentQueue(n int) *segmentQueue {
	if n < 0 {
		n = 0
	}
	return &segmentQueue{
		n:         n,
		taken:     make([]bool, n),
		remaining: n,
	}
}

func (q *segmentQueue) Retarget(i int) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.n == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= q.n {
		i = q.n - 1
	}
	q.next = i
}

func (q *segmentQueue) Stop() {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.stopped = true
	q.mu.Unlock()
}

func (q *segmentQueue) Next() int {
	if q == nil {
		return -1
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped || q.remaining == 0 || q.n == 0 {
		return -1
	}
	for pass := 0; pass < 2; pass++ {
		start, end := 0, q.n
		if pass == 0 {
			start = q.next
		} else {
			end = q.next
		}
		for i := start; i < end; i++ {
			if !q.taken[i] {
				q.taken[i] = true
				q.remaining--
				q.next = i + 1
				return i
			}
		}
	}
	return -1
}

func playBufferReady(bufferEndSec float64, contiguousSegs int) bool {
	return bufferEndSec >= playReadySeconds || contiguousSegs >= playReadySegments
}

func indexForTime(durs []float64, sec float64) int {
	if len(durs) == 0 {
		return 0
	}
	if sec < 0 {
		sec = 0
	}
	acc := 0.0
	for i, d := range durs {
		acc += d
		if sec < acc {
			return i
		}
	}
	return len(durs) - 1
}

func expandTimelineDurations(timeline []*mpd.SegmentTimelineS, timescale uint64) []float64 {
	ts := float64(timescale)
	if ts <= 0 {
		ts = 1
	}
	var out []float64
	for _, s := range timeline {
		if s == nil {
			continue
		}
		d := float64(s.D) / ts
		repeat := int64(0)
		if s.R != nil && *s.R > 0 {
			repeat = *s.R
		}
		for i := int64(0); i < repeat+1; i++ {
			out = append(out, d)
		}
	}
	return out
}

func mediaAfterInit(decrypted []byte) []byte {
	i := 0
	for i+8 <= len(decrypted) {
		size := uint64(binary.BigEndian.Uint32(decrypted[i : i+4]))
		typ := string(decrypted[i+4 : i+8])
		header := 8
		if size == 1 {
			if i+16 > len(decrypted) {
				break
			}
			size = binary.BigEndian.Uint64(decrypted[i+8 : i+16])
			header = 16
		}
		if size < uint64(header) || i+int(size) > len(decrypted) {
			break
		}
		if typ == "moof" {
			return decrypted[i:]
		}
		i += int(size)
	}
	return decrypted
}

func snapshotWidevineKeys() []*widevine.Key {
	if len(keys) == 0 {
		return nil
	}
	out := make([]*widevine.Key, len(keys))
	copy(out, keys)
	return out
}

var (
	playOpenPlayback    = getEpisode
	playParseManifest   = parseManifest
	playGetLicense      = getLicense
	playDownloadPart    = downloadPart
	playDecryptFragment = decryptFragment
	playClosePlayback   = deleteStream
	progressivePlayHeld atomic.Bool
)

func playOpenEpisode(id string) (ep Episode, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = panicAsError(recovered)
		}
	}()
	ep = openPlaybackWithRetry(id, playOpenPlayback, activeConfig.Playback4294Retries, activeConfig.Playback4294Backoff, sleepPlaybackRetry)
	return ep, nil
}

func playParse(url string) (m *mpd.MPD, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = panicAsError(recovered)
		}
	}()
	m = playParseManifest(url)
	return m, nil
}

// StartProgressivePlay opens playback, locks one representation, and downloads
// + decrypts segments into a temp session directory. It returns once the ready
// threshold is met (workers keep filling in the background until Close).
func StartProgressivePlay(ctx context.Context, episodeID string, cfg RuntimeConfig, emit func(PlayProgress)) (*PlaySession, error) {
	if episodeID == "" {
		return nil, errors.New("episode id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	SetRuntimeConfig(cfg)
	if currentJobProgress() != nil {
		return nil, errors.New("download already running")
	}
	if !progressivePlayHeld.CompareAndSwap(false, true) {
		return nil, errors.New("progressive play already running")
	}

	s := &PlaySession{
		EpisodeID:    episodeID,
		VideoQuality: cfg.VideoQuality,
		AudioQuality: cfg.AudioQuality,
		emit:         emit,
		readyCh:      make(chan struct{}, 1),
	}
	if s.VideoQuality == "" {
		s.VideoQuality = activeConfig.VideoQuality
	}
	if s.AudioQuality == "" {
		s.AudioQuality = activeConfig.AudioQuality
	}

	ok := false
	defer func() {
		if !ok {
			_ = s.Close()
		}
	}()

	dir, err := os.MkdirTemp("", "crdl-play-*")
	if err != nil {
		return nil, fmt.Errorf("create play session dir: %w", err)
	}
	s.Dir = dir
	s.playingFile = filepath.Join(dir, playingFileName)
	s.audioFile = filepath.Join(dir, audioFileName)

	episode, err := playOpenEpisode(episodeID)
	if err != nil {
		return nil, err
	}
	s.contentID = episodeID
	s.streamToken = episode.Token

	manifest, err := playParse(episode.ManifestURL)
	if err != nil {
		return nil, err
	}
	if manifest == nil || len(manifest.Period) == 0 || len(manifest.Period[0].AdaptationSets) == 0 {
		return nil, errors.New("manifest has no adaptation sets")
	}

	pssh := getPssh(manifest)
	if pssh == nil {
		return nil, errors.New("PSSH not found")
	}
	if err := playGetLicense(*pssh, episodeID, episode.Token); err != nil {
		return nil, err
	}
	s.keys = snapshotWidevineKeys()

	videoSet := manifest.Period[0].AdaptationSets[0]
	var audioSet *mpd.AdaptationSet
	if len(manifest.Period[0].AdaptationSets) > 1 {
		audioSet = manifest.Period[0].AdaptationSets[1]
	}

	videoBase, videoRep := getBaseUrl(videoSet, true, s.VideoQuality)
	if videoBase == nil || videoRep == nil {
		return nil, errors.New("failed to get the video base URL; check the requested video quality")
	}
	if videoSet.SegmentTemplate == nil || videoSet.SegmentTemplate.Initialization == nil || videoSet.SegmentTemplate.Media == nil || videoSet.SegmentTemplate.SegmentTimeline == nil {
		return nil, errors.New("video segment template missing")
	}

	initURL := buildUrl(*videoBase, *videoRep, *videoSet.SegmentTemplate.Initialization, nil)
	initData, err := playDownloadPart(initURL)
	if err != nil {
		return nil, fmt.Errorf("download video init: %w", err)
	}
	s.videoInit = initData
	if err := os.WriteFile(filepath.Join(dir, initFileName), initData, 0o600); err != nil {
		return nil, err
	}

	s.videoNums = expandTimeline(videoSet.SegmentTemplate.SegmentTimeline.S, 1)
	timescale := uint64(1)
	if videoSet.SegmentTemplate.Timescale != nil {
		timescale = *videoSet.SegmentTemplate.Timescale
	}
	s.segDurations = expandTimelineDurations(videoSet.SegmentTemplate.SegmentTimeline.S, timescale)
	if len(s.segDurations) > len(s.videoNums) {
		s.segDurations = s.segDurations[:len(s.videoNums)]
	}
	for len(s.segDurations) < len(s.videoNums) {
		s.segDurations = append(s.segDurations, 0)
	}
	n := len(s.videoNums)
	if n == 0 {
		return nil, errors.New("video timeline has no segments")
	}
	for _, d := range s.segDurations {
		s.durationSec += d
	}
	s.queue = newSegmentQueue(n)
	s.have = make([]bool, n)
	s.pending = make([][]byte, n)

	var audioBase, audioRep *string
	if audioSet != nil && audioSet.SegmentTemplate != nil && audioSet.SegmentTemplate.Initialization != nil && audioSet.SegmentTemplate.Media != nil && audioSet.SegmentTemplate.SegmentTimeline != nil {
		audioBase, audioRep = getBaseUrl(audioSet, false, s.AudioQuality)
		if audioBase != nil && audioRep != nil {
			aInitURL := buildUrl(*audioBase, *audioRep, *audioSet.SegmentTemplate.Initialization, nil)
			aInit, aerr := playDownloadPart(aInitURL)
			if aerr == nil {
				s.audioInit = aInit
				s.audioNums = expandTimeline(audioSet.SegmentTemplate.SegmentTimeline.S, 1)
				ats := uint64(1)
				if audioSet.SegmentTemplate.Timescale != nil {
					ats = *audioSet.SegmentTemplate.Timescale
				}
				s.audioDurations = expandTimelineDurations(audioSet.SegmentTemplate.SegmentTimeline.S, ats)
				an := len(s.audioNums)
				if an > 0 {
					s.audioQueue = newSegmentQueue(an)
					s.audioHave = make([]bool, an)
					s.audioPending = make([][]byte, an)
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	errCh := make(chan error, 2)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := runQueuedSegmentJobs(ctx, s.queue, playVideoWorkers, func(index int) error {
			if index < 0 || index >= len(s.videoNums) {
				return fmt.Errorf("video segment index %d out of range", index)
			}
			num := s.videoNums[index]
			url := buildUrl(*videoBase, *videoRep, *videoSet.SegmentTemplate.Media, &num)
			enc, err := playDownloadPart(url)
			if err != nil {
				return err
			}
			dec, err := playDecryptFragment(s.videoInit, enc, s.keys)
			if err != nil {
				return err
			}
			return s.commitVideoSegment(index, dec)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case errCh <- err:
			default:
			}
			s.emitErr(err)
		}
	}()

	if s.audioQueue != nil && audioSet != nil && audioBase != nil && audioRep != nil {
		aSet, aBase, aRep := audioSet, audioBase, audioRep
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := runQueuedSegmentJobs(ctx, s.audioQueue, playAudioWorkers, func(index int) error {
				if index < 0 || index >= len(s.audioNums) {
					return fmt.Errorf("audio segment index %d out of range", index)
				}
				num := s.audioNums[index]
				url := buildUrl(*aBase, *aRep, *aSet.SegmentTemplate.Media, &num)
				enc, err := playDownloadPart(url)
				if err != nil {
					return err
				}
				dec, err := playDecryptFragment(s.audioInit, enc, s.keys)
				if err != nil {
					return err
				}
				return s.commitAudioSegment(index, dec)
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				s.emitErr(err)
			}
		}()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-s.readyCh:
		ok = true
		return s, nil
	}
}

func (s *PlaySession) emitErr(err error) {
	if s == nil || s.emit == nil || err == nil {
		return
	}
	s.mu.Lock()
	p := PlayProgress{BufferEndSec: s.BufferEndSec, DurationSec: s.durationSec, PlayingPath: s.playingFile, Err: err}
	s.mu.Unlock()
	s.emit(p)
}

func (s *PlaySession) commitVideoSegment(index int, decrypted []byte) error {
	if s == nil {
		return errors.New("nil play session")
	}
	path := filepath.Join(s.Dir, fmt.Sprintf("seg-%04d.m4s", index))
	if err := os.WriteFile(path, mediaAfterInit(decrypted), 0o600); err != nil {
		return err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	if index >= 0 && index < len(s.have) {
		s.have[index] = true
		s.pending[index] = decrypted
	}
	var appendErr error
	for s.contiguous < len(s.have) && s.have[s.contiguous] {
		if err := s.appendPlayingLocked(s.contiguous, s.pending[s.contiguous], s.playingFile); err != nil {
			appendErr = err
			break
		}
		s.pending[s.contiguous] = nil
		if s.contiguous < len(s.segDurations) {
			s.BufferEndSec += s.segDurations[s.contiguous]
		}
		s.contiguous++
	}
	ready := playBufferReady(s.BufferEndSec, s.contiguous) || (len(s.have) > 0 && s.contiguous >= len(s.have))
	firstReady := ready && !s.readyEmitted
	if firstReady {
		s.readyEmitted = true
	}
	progress := PlayProgress{
		BufferEndSec: s.BufferEndSec,
		DurationSec:  s.durationSec,
		Ready:        firstReady,
		PlayingPath:  s.playingFile,
	}
	emit := s.emit
	s.mu.Unlock()

	if appendErr != nil {
		return appendErr
	}
	if emit != nil {
		emit(progress)
	}
	if firstReady {
		select {
		case s.readyCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (s *PlaySession) commitAudioSegment(index int, decrypted []byte) error {
	if s == nil {
		return errors.New("nil play session")
	}
	path := filepath.Join(s.Dir, fmt.Sprintf("audio-seg-%04d.m4s", index))
	if err := os.WriteFile(path, mediaAfterInit(decrypted), 0o600); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if index >= 0 && index < len(s.audioHave) {
		s.audioHave[index] = true
		s.audioPending[index] = decrypted
	}
	for s.audioContiguous < len(s.audioHave) && s.audioHave[s.audioContiguous] {
		if err := s.appendPlayingLocked(s.audioContiguous, s.audioPending[s.audioContiguous], s.audioFile); err != nil {
			return err
		}
		s.audioPending[s.audioContiguous] = nil
		s.audioContiguous++
	}
	return nil
}

func (s *PlaySession) appendPlayingLocked(index int, decrypted []byte, dest string) error {
	data := decrypted
	if index > 0 {
		data = mediaAfterInit(decrypted)
	}
	flag := os.O_WRONLY | os.O_CREATE
	if index > 0 {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(dest, flag, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// SeekTarget maps seconds to a segment index and prioritizes that index in the
// worker queue. Quality is never changed.
func (s *PlaySession) SeekTarget(sec float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	vq, aq := s.queue, s.audioQueue
	vd, ad := s.segDurations, s.audioDurations
	s.mu.Unlock()
	if vq != nil {
		vq.Retarget(indexForTime(vd, sec))
	}
	if aq != nil {
		aq.Retarget(indexForTime(ad, sec))
	}
}

func (s *PlaySession) BufferedEnd() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.BufferEndSec
}

func (s *PlaySession) Duration() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.durationSec
}

func (s *PlaySession) PlayingPath() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.playingFile != "" {
		return s.playingFile
	}
	if s.Dir == "" {
		return ""
	}
	return filepath.Join(s.Dir, playingFileName)
}

func (s *PlaySession) AudioPath() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.audioContiguous <= 0 {
		return ""
	}
	return s.audioFile
}

// Close cancels workers, releases the playback stream, and removes the temp dir.
func (s *PlaySession) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	cancel := s.cancel
	q := s.queue
	aq := s.audioQueue
	dir := s.Dir
	token := s.streamToken
	id := s.contentID
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if q != nil {
		q.Stop()
	}
	if aq != nil {
		aq.Stop()
	}
	s.wg.Wait()
	if token != "" && playClosePlayback != nil {
		_ = playClosePlayback(id, token)
	}
	var err error
	if dir != "" {
		err = os.RemoveAll(dir)
	}
	progressivePlayHeld.Store(false)
	return err
}
