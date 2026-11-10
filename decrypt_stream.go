package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/mp4"
)

// decryptIOBufferSize is the size of the read and write buffers used while
// decrypting. mp4ff decodes box headers and fields with many small reads, so
// buffering saves a syscall per field while keeping memory use fixed.
const decryptIOBufferSize = 1 << 20

// decryptFragmentedMP4 decrypts the fragmented MP4 read from r with key and
// writes the clear result to w, one fragment at a time. CENC and CBCS are
// supported, through mp4ff's DecryptInit and DecryptFragment.
//
// It replaces gowidevine's DecryptMP4, which decodes the whole file with
// mp4.DecodeFile before writing anything and therefore holds every mdat
// payload in memory at once: a 5 GB movie needed more than 5 GB of RAM. Here
// only the current moof+mdat pair is resident, so memory use is bounded by the
// largest fragment instead of growing with the length of the title.
//
// The output follows the same rules as DecryptMP4:
//   - the init segment is written with its protection boxes removed;
//   - styp and emsg boxes are copied through in order;
//   - sidx, mfra and any other top-level boxes are dropped, as their offsets
//     no longer hold once the encryption boxes are removed from each moof;
//   - a fragment whose protected tracks carry no senc box is clear and is
//     copied through unchanged.
func decryptFragmentedMP4(r io.Reader, key []byte, w io.Writer) (err error) {
	// mp4ff indexes slices with sizes taken straight from the file (sample
	// sizes in trun, for instance), so malformed input can make it panic.
	// Report that as an error instead of crashing the download.
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("malformed MP4: %v", p)
		}
	}()

	// Absolute input offsets are needed to resolve trun data offsets, which
	// count from the start of the moof. Count what is actually consumed rather
	// than trusting the re-encoded size of each decoded box.
	var offset uint64
	in := &countingReader{
		r:      bufio.NewReaderSize(r, decryptIOBufferSize),
		onRead: func(n int) { offset += uint64(n) },
	}
	out := bufio.NewWriterSize(w, decryptIOBufferSize)

	var (
		ftyp        *mp4.FtypBox
		decryptInfo *mp4.DecryptInfo // set once the init segment is written
		moof        *mp4.MoofBox     // waiting for its mdat
	)
	for {
		start := offset
		box, err := mp4.DecodeBox(start, in)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("decode box at offset %d: %w", start, err)
		}
		if moof != nil && box.Type() != "mdat" {
			return fmt.Errorf("moof at offset %d is followed by %s instead of mdat", moof.StartPos, box.Type())
		}

		switch b := box.(type) {
		case *mp4.FtypBox:
			if decryptInfo == nil {
				ftyp = b
			}
		case *mp4.MoovBox:
			if decryptInfo != nil {
				return fmt.Errorf("second moov box at offset %d", start)
			}
			info, err := writeClearInit(ftyp, b, out)
			if err != nil {
				return err
			}
			decryptInfo = &info
		case *mp4.MoofBox:
			if decryptInfo == nil {
				return fmt.Errorf("moof at offset %d comes before the moov box", start)
			}
			b.StartPos = start
			moof = b
		case *mp4.MdatBox:
			if moof == nil {
				return fmt.Errorf("mdat at offset %d is not preceded by a moof", start)
			}
			frag := mp4.NewFragment()
			frag.AddChild(moof)
			frag.AddChild(b)
			moof = nil
			if err := decryptFragment(frag, *decryptInfo, key); err != nil {
				return fmt.Errorf("decrypt fragment at offset %d: %w", frag.Moof.StartPos, err)
			}
			if err := frag.Encode(out); err != nil {
				return fmt.Errorf("write fragment at offset %d: %w", frag.Moof.StartPos, err)
			}
			// frag goes out of scope here, releasing the samples.
		case *mp4.StypBox, *mp4.EmsgBox, *mp4.PrftBox:
			if decryptInfo == nil {
				return fmt.Errorf("%s at offset %d comes before the moov box", box.Type(), start)
			}
			if err := box.Encode(out); err != nil {
				return fmt.Errorf("write %s at offset %d: %w", box.Type(), start, err)
			}
		}
		// Every other top-level box (sidx, mfra, free, ...) is dropped.
	}

	if moof != nil {
		return fmt.Errorf("input ends after the moof at offset %d, before its mdat", moof.StartPos)
	}
	if decryptInfo == nil {
		return errors.New("no init part of file")
	}
	if err := out.Flush(); err != nil {
		return fmt.Errorf("write decrypted MP4: %w", err)
	}
	return nil
}

// writeClearInit strips the protection boxes from the init segment and writes
// it to w, returning what is needed to decrypt the fragments that follow.
func writeClearInit(ftyp *mp4.FtypBox, moov *mp4.MoovBox, w io.Writer) (mp4.DecryptInfo, error) {
	if moov.Mvex == nil {
		return mp4.DecryptInfo{}, errors.New("file is not fragmented")
	}
	init := mp4.NewMP4Init()
	if ftyp != nil {
		init.AddChild(ftyp)
	}
	init.AddChild(moov)

	info, err := mp4.DecryptInit(init)
	if err != nil {
		return info, fmt.Errorf("read MP4 encryption info: %w", err)
	}
	if err := init.Encode(w); err != nil {
		return info, fmt.Errorf("write init segment: %w", err)
	}
	return info, nil
}

// decryptFragment decrypts frag in place.
//
// A fragment whose protected tracks have no senc box is in the clear, as in an
// unencrypted lead-in, and is left untouched like gowidevine did (see
// https://github.com/iyear/gowidevine/pull/26#issuecomment-2385960551). That
// is decided before calling mp4ff, because DecryptFragment can return the
// "no senc box" error after it has already modified an earlier track.
func decryptFragment(frag *mp4.Fragment, info mp4.DecryptInfo, key []byte) error {
	var withSenc, withoutSenc int
	for _, traf := range frag.Moof.Trafs {
		if traf.Tfhd == nil {
			return errors.New("traf box missing tfhd child")
		}
		if !isProtectedTrack(info, traf.Tfhd.TrackID) {
			continue
		}
		if hasSenc, _ := traf.ContainsSencBox(); hasSenc {
			withSenc++
		} else {
			withoutSenc++
		}
	}
	switch {
	case withSenc == 0:
		return nil
	case withoutSenc > 0:
		return errors.New("fragment mixes protected tracks with and without senc boxes")
	}
	return mp4.DecryptFragment(frag, info, key)
}

// isProtectedTrack reports whether the init segment declared trackID as
// encrypted. It uses the first entry for the track, like mp4ff does.
func isProtectedTrack(info mp4.DecryptInfo, trackID uint32) bool {
	for _, track := range info.TrackInfos {
		if track.TrackID == trackID {
			return track.Sinf != nil && track.Sinf.Schi != nil && track.Sinf.Schi.Tenc != nil
		}
	}
	return false
}
