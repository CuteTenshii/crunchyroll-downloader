package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
)

func findMp4ffTestData(t testing.TB) string {
	t.Helper()
	if custom := os.Getenv("MP4FF_TESTDATA"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom
		}
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Eyevinn/mp4ff")
	if out, err := cmd.Output(); err == nil {
		dir := strings.TrimSpace(string(out))
		testDataDir := filepath.Join(dir, "mp4", "testdata")
		if _, err := os.Stat(testDataDir); err == nil {
			return testDataDir
		}
	}
	candidates := []string{
		`C:\Users\Izzaldin\go\pkg\mod\github.com\!eyevinn\mp4ff@v0.48.0\mp4\testdata`,
		`/mnt/c/Users/Izzaldin/go/pkg/mod/github.com/!eyevinn/mp4ff@v0.48.0/mp4/testdata`,
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "go", "pkg", "mod", "github.com", "!eyevinn", "mp4ff@v0.48.0", "mp4", "testdata"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skip("mp4ff testdata dir not found")
	return ""
}

// TestDecryptFragmentedMP4_ReferenceFiles tests exact match against mp4ff reference
// files for CENC and CBCS schemes, comparing with both gowidevine.DecryptMP4 and expected files.
func TestDecryptFragmentedMP4_ReferenceFiles(t *testing.T) {
	testDataDir := findMp4ffTestData(t)

	testCases := []struct {
		name        string
		inFile      string
		expectedOut string
		keyHex      string
	}{
		{
			name:        "cenc_dashinit",
			inFile:      filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4"),
			expectedOut: filepath.Join(testDataDir, "prog_8s_dec_dashinit.mp4"),
			keyHex:      "63cb5f7184dd4b689a5c5ff11ee6a328",
		},
		{
			name:        "cbcs_video",
			inFile:      filepath.Join(testDataDir, "cbcs.mp4"),
			expectedOut: filepath.Join(testDataDir, "cbcsdec.mp4"),
			keyHex:      "22bdb0063805260307ee5045c0f3835a",
		},
		{
			name:        "cbcs_audio",
			inFile:      filepath.Join(testDataDir, "cbcs_audio.mp4"),
			expectedOut: filepath.Join(testDataDir, "cbcs_audiodec.mp4"),
			keyHex:      "5ffd93861fa776e96cccd934898fc1c8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inData, err := os.ReadFile(tc.inFile)
			if err != nil {
				t.Fatalf("read input file %s: %v", tc.inFile, err)
			}
			expectedData, err := os.ReadFile(tc.expectedOut)
			if err != nil {
				t.Fatalf("read expected file %s: %v", tc.expectedOut, err)
			}

			key, err := hex.DecodeString(tc.keyHex)
			if err != nil {
				t.Fatalf("decode key: %v", err)
			}

			// 1. Run legacy gowidevine.DecryptMP4 for direct comparison
			var legacyOut bytes.Buffer
			legacyErr := widevine.DecryptMP4(bytes.NewReader(inData), key, &legacyOut)
			if legacyErr != nil {
				t.Fatalf("legacy DecryptMP4 failed: %v", legacyErr)
			}

			// 2. Run new streaming decryptFragmentedMP4
			var streamOut bytes.Buffer
			streamErr := decryptFragmentedMP4(bytes.NewReader(inData), key, &streamOut)
			if streamErr != nil {
				t.Fatalf("decryptFragmentedMP4 failed: %v", streamErr)
			}

			// 3. Verify output matches expected reference and legacy output
			streamBytes := streamOut.Bytes()
			legacyBytes := legacyOut.Bytes()

			if !bytes.Equal(streamBytes, legacyBytes) {
				t.Errorf("streaming output differs from legacy output: len %d vs %d", len(streamBytes), len(legacyBytes))
			}

			expectedSum := sha256.Sum256(expectedData)
			streamSum := sha256.Sum256(streamBytes)
			if streamSum != expectedSum {
				t.Errorf("streaming output hash %x != expected reference hash %x", streamSum, expectedSum)
			}
		})
	}
}

// TestDecryptMP4_ContentKeySelection verifies that decryptMP4 correctly matches
// the track's default KID to the license key.
func TestDecryptMP4_ContentKeySelection(t *testing.T) {
	testDataDir := findMp4ffTestData(t)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	// Extract init segment
	parsed, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode file: %v", err)
	}
	var initBuf bytes.Buffer
	if err := parsed.Init.Encode(&initBuf); err != nil {
		t.Fatalf("encode init: %v", err)
	}
	initData := initBuf.Bytes()

	di, err := mp4.DecryptInit(parsed.Init)
	if err != nil {
		t.Fatalf("decrypt init: %v", err)
	}
	kid := di.TrackInfos[0].Sinf.Schi.Tenc.DefaultKID
	correctKey, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")
	dummyKey, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	wrongKID := []byte("wrong_kid_123456")

	t.Run("matching key among multiple keys", func(t *testing.T) {
		keys := []*widevine.Key{
			{Type: widevinepb.License_KeyContainer_SIGNING, ID: kid, Key: dummyKey},
			{Type: widevinepb.License_KeyContainer_CONTENT, ID: wrongKID, Key: dummyKey},
			{Type: widevinepb.License_KeyContainer_CONTENT, ID: kid, Key: correctKey},
			{Type: widevinepb.License_KeyContainer_CONTENT, ID: []byte("other_kid_999999"), Key: dummyKey},
		}

		var out bytes.Buffer
		err := decryptMP4(initData, bytes.NewReader(data), keys, &out)
		if err != nil {
			t.Fatalf("decryptMP4 failed: %v", err)
		}
		if out.Len() == 0 {
			t.Fatal("decryptMP4 produced empty output")
		}
	})

	t.Run("no matching content key", func(t *testing.T) {
		keys := []*widevine.Key{
			{Type: widevinepb.License_KeyContainer_CONTENT, ID: wrongKID, Key: dummyKey},
		}
		var out bytes.Buffer
		err := decryptMP4(initData, bytes.NewReader(data), keys, &out)
		if err == nil {
			t.Fatal("expected error for missing content key, got nil")
		}
		if !strings.Contains(err.Error(), "no license key found") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("no encrypted tracks", func(t *testing.T) {
		decFile := filepath.Join(testDataDir, "prog_8s_dec_dashinit.mp4")
		decData, err := os.ReadFile(decFile)
		if err != nil {
			t.Fatalf("read dec file: %v", err)
		}
		decParsed, err := mp4.DecodeFile(bytes.NewReader(decData))
		if err != nil {
			t.Fatalf("decode dec file: %v", err)
		}
		var decInitBuf bytes.Buffer
		if err := decParsed.Init.Encode(&decInitBuf); err != nil {
			t.Fatalf("encode dec init: %v", err)
		}

		var out bytes.Buffer
		err = decryptMP4(decInitBuf.Bytes(), bytes.NewReader(decData), nil, &out)
		if err == nil {
			t.Fatal("expected error for unencrypted init, got nil")
		}
		if !strings.Contains(err.Error(), "no encrypted tracks") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("corrupt init data", func(t *testing.T) {
		var out bytes.Buffer
		err := decryptMP4([]byte("not_mp4"), bytes.NewReader(data), nil, &out)
		if err == nil {
			t.Fatal("expected error for corrupt init, got nil")
		}
	})
}

// TestDecryptFragmentedMP4_UnencryptedFragments verifies that fragments without
// senc boxes (such as unencrypted pre-roll) pass through unmodified while subsequent
// encrypted fragments are properly decrypted.
func TestDecryptFragmentedMP4_UnencryptedFragments(t *testing.T) {
	testDataDir := findMp4ffTestData(t)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	parsed, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode file: %v", err)
	}

	// Create an MP4 with init + unencrypted fragment + encrypted fragment.
	encFrag := parsed.Segments[0].Fragments[0]
	unencFrag := mp4.NewFragment()
	// Deep clone moof
	var moofBuf bytes.Buffer
	if err := encFrag.Moof.Encode(&moofBuf); err != nil {
		t.Fatalf("encode moof: %v", err)
	}
	moofBox, err := mp4.DecodeBox(0, &moofBuf)
	if err != nil {
		t.Fatalf("decode cloned moof: %v", err)
	}
	clonedMoof := moofBox.(*mp4.MoofBox)
	// Remove encryption boxes from traf so it has no senc
	for _, traf := range clonedMoof.Trafs {
		_ = traf.RemoveEncryptionBoxes()
	}
	clonedMoof.RemovePsshs()
	unencFrag.AddChild(clonedMoof)

	// Add dummy clear mdat
	clearMdatData := make([]byte, encFrag.Mdat.DataLength())
	for i := range clearMdatData {
		clearMdatData[i] = byte(i % 251)
	}
	unencMdat := &mp4.MdatBox{Data: clearMdatData}
	unencFrag.AddChild(unencMdat)

	// Build composite file: Init + unencFrag + encFrag
	var compositeBuf bytes.Buffer
	if err := parsed.Init.Encode(&compositeBuf); err != nil {
		t.Fatalf("encode init: %v", err)
	}
	if err := unencFrag.Encode(&compositeBuf); err != nil {
		t.Fatalf("encode unenc frag: %v", err)
	}
	if err := encFrag.Encode(&compositeBuf); err != nil {
		t.Fatalf("encode enc frag: %v", err)
	}

	key, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")

	var decryptedBuf bytes.Buffer
	err = decryptFragmentedMP4(bytes.NewReader(compositeBuf.Bytes()), key, &decryptedBuf)
	if err != nil {
		t.Fatalf("decryptFragmentedMP4 failed: %v", err)
	}

	// Verify the output has 2 fragments, and unencrypted fragment's mdat data is intact
	decParsed, err := mp4.DecodeFile(bytes.NewReader(decryptedBuf.Bytes()))
	if err != nil {
		t.Fatalf("decode decrypted output: %v", err)
	}
	if len(decParsed.Segments) == 0 || len(decParsed.Segments[0].Fragments) != 2 {
		t.Fatalf("expected 2 fragments in output, got %d segments", len(decParsed.Segments))
	}
	frag0Mdat := decParsed.Segments[0].Fragments[0].Mdat.Data
	if !bytes.Equal(frag0Mdat, clearMdatData) {
		t.Errorf("unencrypted fragment data was altered during decryption")
	}
}

// TestDecryptFragmentedMP4_MalformedAndErrors verifies robust error handling.
func TestDecryptFragmentedMP4_MalformedAndErrors(t *testing.T) {
	key := make([]byte, 16)

	t.Run("non-fragmented MP4", func(t *testing.T) {
		init := mp4.NewMP4Init()
		init.AddChild(mp4.CreateFtyp())
		init.AddChild(&mp4.MoovBox{}) // No mvex
		var buf bytes.Buffer
		_ = init.Encode(&buf)

		err := decryptFragmentedMP4(&buf, key, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "not fragmented") {
			t.Fatalf("expected 'not fragmented' error, got %v", err)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		err := decryptFragmentedMP4(bytes.NewReader(nil), key, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "no init part") {
			t.Fatalf("expected 'no init part' error, got %v", err)
		}
	})

	t.Run("moof before moov", func(t *testing.T) {
		testDataDir := findMp4ffTestData(t)
		cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
		data, err := os.ReadFile(cencFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		parsed, err := mp4.DecodeFile(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode file: %v", err)
		}
		var buf bytes.Buffer
		if err := parsed.Segments[0].Fragments[0].Moof.Encode(&buf); err != nil {
			t.Fatalf("encode moof: %v", err)
		}

		err = decryptFragmentedMP4(&buf, key, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "comes before the moov") {
			t.Fatalf("expected 'comes before the moov' error, got %v", err)
		}
	})

	t.Run("truncated header", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x00, 0x20, 'f', 't'} // only 6 bytes of an 8 byte header
		err := decryptFragmentedMP4(bytes.NewReader(data), key, io.Discard)
		if err == nil {
			t.Fatal("expected error on truncated header, got nil")
		}
	})

	t.Run("writer failure propagates", func(t *testing.T) {
		testDataDir := findMp4ffTestData(t)
		cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
		data, err := os.ReadFile(cencFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		cencKey, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")

		failErr := errors.New("disk write failure")
		failWriter := &errWriter{failAfterBytes: 50, err: failErr}

		err = decryptFragmentedMP4(bytes.NewReader(data), cencKey, failWriter)
		if err == nil {
			t.Fatal("expected write error, got nil")
		}
		if !errors.Is(err, failErr) && !strings.Contains(err.Error(), failErr.Error()) {
			t.Fatalf("expected error containing %v, got %v", failErr, err)
		}
	})

	t.Run("traf missing tfhd", func(t *testing.T) {
		testDataDir := findMp4ffTestData(t)
		cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
		data, err := os.ReadFile(cencFile)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		parsed, err := mp4.DecodeFile(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode file: %v", err)
		}

		// Create a fragment with a traf that has no tfhd
		badFrag := mp4.NewFragment()
		badMoof := &mp4.MoofBox{}
		badTraf := &mp4.TrafBox{} // No Tfhd
		_ = badMoof.AddChild(badTraf)
		badFrag.AddChild(badMoof)
		badFrag.AddChild(&mp4.MdatBox{Data: []byte{1, 2, 3}})

		var buf bytes.Buffer
		_ = parsed.Init.Encode(&buf)
		_ = badMoof.Encode(&buf)
		_ = badFrag.Mdat.Encode(&buf)

		cencKey, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")
		err = decryptFragmentedMP4(&buf, cencKey, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "missing tfhd") {
			t.Fatalf("expected error mentioning missing tfhd, got %v", err)
		}
	})
}

type errWriter struct {
	written        int
	failAfterBytes int
	err            error
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.written >= w.failAfterBytes {
		return 0, w.err
	}
	avail := w.failAfterBytes - w.written
	if len(p) > avail {
		w.written += avail
		return avail, w.err
	}
	w.written += len(p)
	return len(p), nil
}

// trackingReader counts bytes read.
type trackingReader struct {
	r     io.Reader
	total int64
}

func (tr *trackingReader) Read(p []byte) (int, error) {
	n, err := tr.r.Read(p)
	tr.total += int64(n)
	return n, err
}

// trackingWriter counts bytes written and checks in-flight gap.
type trackingWriter struct {
	w           io.Writer
	tr          *trackingReader
	written     int64
	maxInFlight int64
	checkCount  int
}

func (tw *trackingWriter) Write(p []byte) (int, error) {
	n, err := tw.w.Write(p)
	tw.written += int64(n)
	inFlight := tw.tr.total - tw.written
	if inFlight > tw.maxInFlight {
		tw.maxInFlight = inFlight
	}
	tw.checkCount++
	return n, err
}

// TestDecryptFragmentedMP4_StreamingBoundedMemory verifies that memory usage does not
// grow with input size: input read-ahead minus output written stays bounded to buffer sizes.
func TestDecryptFragmentedMP4_StreamingBoundedMemory(t *testing.T) {
	testDataDir := findMp4ffTestData(t)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	parsed, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode file: %v", err)
	}

	// Build a multi-fragment stream by repeating the fragment 30 times
	var streamInput bytes.Buffer
	if err := parsed.Init.Encode(&streamInput); err != nil {
		t.Fatalf("encode init: %v", err)
	}
	frag := parsed.Segments[0].Fragments[0]
	for i := 0; i < 30; i++ {
		if err := frag.Encode(&streamInput); err != nil {
			t.Fatalf("encode fragment %d: %v", i, err)
		}
	}
	totalInputSize := int64(streamInput.Len())

	t.Logf("Total synthetic input size: %d bytes (30 fragments)", totalInputSize)

	key, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")

	tr := &trackingReader{r: &streamInput}
	tw := &trackingWriter{w: io.Discard, tr: tr}

	err = decryptFragmentedMP4(tr, key, tw)
	if err != nil {
		t.Fatalf("decryptFragmentedMP4 failed: %v", err)
	}

	t.Logf("Max in-flight bytes (read - written): %d bytes across %d writes", tw.maxInFlight, tw.checkCount)

	// Since decryptIOBufferSize is 1 MiB, max in-flight (buffered in reader + writer)
	// should be well below 5 MiB, nowhere near totalInputSize.
	maxAllowedInFlight := int64(4 * decryptIOBufferSize)
	if tw.maxInFlight > maxAllowedInFlight {
		t.Errorf("max in-flight bytes %d exceeded bound %d", tw.maxInFlight, maxAllowedInFlight)
	}
}

// repeatingFragmentReader generates a valid fragmented MP4 stream of arbitrary size
// on the fly by repeating fragData, requiring only O(1) memory for generation.
type repeatingFragmentReader struct {
	initData []byte
	fragData []byte
	count    int
	current  int
	offset   int
}

func (r *repeatingFragmentReader) Read(p []byte) (int, error) {
	if r.current == 0 {
		n := copy(p, r.initData[r.offset:])
		r.offset += n
		if r.offset >= len(r.initData) {
			r.current = 1
			r.offset = 0
		}
		return n, nil
	}
	if r.current > r.count {
		return 0, io.EOF
	}
	n := copy(p, r.fragData[r.offset:])
	r.offset += n
	if r.offset >= len(r.fragData) {
		r.current++
		r.offset = 0
	}
	return n, nil
}

// TestDecryptFragmentedMP4_MultiFragment_Playable verifies that multi-fragment
// decrypted output is structurally valid and playable with ffprobe.
func TestDecryptFragmentedMP4_MultiFragment_Playable(t *testing.T) {
	candidates := []string{
		"ffprobe",
		`C:\ProgramData\chocolatey\bin\ffprobe.exe`,
		`/mnt/c/ProgramData/chocolatey/bin/ffprobe.exe`,
	}
	var ffprobePath string
	for _, c := range candidates {
		if runtime.GOOS == "linux" && strings.HasSuffix(c, ".exe") {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			ffprobePath = path
			break
		}
		if _, err := os.Stat(c); err == nil {
			ffprobePath = c
			break
		}
	}
	if ffprobePath == "" {
		t.Skip("ffprobe not available in PATH")
	}

	testDataDir := findMp4ffTestData(t)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	parsed, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode file: %v", err)
	}

	var initBuf bytes.Buffer
	if err := parsed.Init.Encode(&initBuf); err != nil {
		t.Fatalf("encode init: %v", err)
	}
	var fragBuf bytes.Buffer
	frag := parsed.Segments[0].Fragments[0]
	if err := frag.Encode(&fragBuf); err != nil {
		t.Fatalf("encode fragment: %v", err)
	}

	// 8 sequential fragments
	r := &repeatingFragmentReader{
		initData: initBuf.Bytes(),
		fragData: fragBuf.Bytes(),
		count:    8,
	}

	tmpFile := filepath.Join(t.TempDir(), "playable_dec.mp4")
	outFile, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer outFile.Close()

	key, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")
	if err := decryptFragmentedMP4(r, key, outFile); err != nil {
		t.Fatalf("decryptFragmentedMP4 failed: %v", err)
	}
	_ = outFile.Close()

	// Verify using ffprobe
	cmd := exec.Command(ffprobePath, "-v", "error", "-show_entries", "format=format_name,duration:stream=codec_name", "-of", "default=noprint_wrappers=1", tmpFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe failed on decrypted output: %v\nOutput: %s", err, string(out))
	}
	probeStr := string(out)
	t.Logf("ffprobe output:\n%s", probeStr)
	if !strings.Contains(probeStr, "codec_name=h264") {
		t.Errorf("expected codec_name=h264 in ffprobe output, got:\n%s", probeStr)
	}
	if !strings.Contains(probeStr, "format_name=mov,mp4,m4a,3gp,3g2,mj2") {
		t.Errorf("expected mp4 format in ffprobe output, got:\n%s", probeStr)
	}
}

// TestDecryptFragmentedMP4_512MiB_MemoryBounded tests streaming decryption over a 512+ MiB
// fragmented MP4 stream and verifies that peak memory remains bounded well below 256 MiB.
func TestDecryptFragmentedMP4_512MiB_MemoryBounded(t *testing.T) {
	testDataDir := findMp4ffTestData(t)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	parsed, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode file: %v", err)
	}

	var initBuf bytes.Buffer
	if err := parsed.Init.Encode(&initBuf); err != nil {
		t.Fatalf("encode init: %v", err)
	}
	var fragBuf bytes.Buffer
	frag := parsed.Segments[0].Fragments[0]
	if err := frag.Encode(&fragBuf); err != nil {
		t.Fatalf("encode fragment: %v", err)
	}

	fragLen := len(fragBuf.Bytes())
	// 512 MiB = 536,870,912 bytes.
	// With ~116 KiB per fragment, 4650 fragments = ~540 MiB
	targetCount := (512*1024*1024)/fragLen + 10
	totalStreamBytes := int64(len(initBuf.Bytes()) + targetCount*fragLen)

	t.Logf("Streaming %d fragments (%d bytes / %.2f MiB)", targetCount, totalStreamBytes, float64(totalStreamBytes)/(1024*1024))

	r := &repeatingFragmentReader{
		initData: initBuf.Bytes(),
		fragData: fragBuf.Bytes(),
		count:    targetCount,
	}

	key, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")

	// Measure memory before, during, and after
	var mBefore, mAfter, mPeak runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	// Run streaming decryption to a hash sink
	hasher := sha256.New()
	tw := &memTrackingWriter{
		w: hasher,
		onTick: func() {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > mPeak.HeapAlloc {
				mPeak = m
			}
		},
	}

	err = decryptFragmentedMP4(r, key, tw)
	if err != nil {
		t.Fatalf("decryptFragmentedMP4 failed: %v", err)
	}

	runtime.ReadMemStats(&mAfter)

	peakHeapMiB := float64(mPeak.HeapAlloc) / (1024 * 1024)
	peakSysMiB := float64(mPeak.HeapSys) / (1024 * 1024)
	totalAllocMiB := float64(mAfter.TotalAlloc-mBefore.TotalAlloc) / (1024 * 1024)

	t.Logf("Result: 512+ MiB Decryption completed successfully!")
	t.Logf("Decrypted stream SHA256: %x", hasher.Sum(nil))
	t.Logf("Peak Live HeapAlloc: %.2f MiB (target: < 64 MiB, limit: < 256 MiB)", peakHeapMiB)
	t.Logf("Peak HeapSys:       %.2f MiB", peakSysMiB)
	t.Logf("Cumulative Alloc:   %.2f MiB", totalAllocMiB)

	// Acceptance criteria: Peak live heap memory must be strictly bounded (< 64 MiB, well below 256 MiB)
	if mPeak.HeapAlloc > 64*1024*1024 {
		t.Errorf("peak HeapAlloc %.2f MiB exceeded 64 MiB budget", peakHeapMiB)
	}
}

type memTrackingWriter struct {
	w      io.Writer
	count  int
	onTick func()
}

func (mw *memTrackingWriter) Write(p []byte) (int, error) {
	mw.count++
	if mw.count%100 == 0 && mw.onTick != nil {
		mw.onTick()
	}
	return mw.w.Write(p)
}

func BenchmarkDecryptFragmentedMP4(b *testing.B) {
	testDataDir := findMp4ffTestData(b)
	cencFile := filepath.Join(testDataDir, "prog_8s_enc_dashinit.mp4")
	data, err := os.ReadFile(cencFile)
	if err != nil {
		b.Fatalf("read file: %v", err)
	}
	key, _ := hex.DecodeString("63cb5f7184dd4b689a5c5ff11ee6a328")

	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := decryptFragmentedMP4(bytes.NewReader(data), key, io.Discard)
		if err != nil {
			b.Fatalf("decrypt failed: %v", err)
		}
	}
}
