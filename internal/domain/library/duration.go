package library

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DurationFromFile extracts duration in milliseconds from the audio container header.
// Returns 0 when unsupported or unreadable.
func DurationFromFile(path string) int64 {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 12)
	if _, err := io.ReadFull(f, magic); err != nil {
		return 0
	}

	switch {
	case string(magic[:4]) == "fLaC":
		// flacDuration reads the first metadata block header starting at byte 4
		f.Seek(4, io.SeekStart) //nolint:errcheck
		return flacDuration(f)
	case string(magic[:4]) == "RIFF" && string(magic[8:12]) == "WAVE":
		// WAV chunks begin at byte 12 — position already correct
		return wavDuration(f)
	case string(magic[:4]) == "FORM":
		// aiffDuration reads FORM size (bytes 4-7) then type (bytes 8-11)
		f.Seek(4, io.SeekStart) //nolint:errcheck
		return aiffDuration(f)
	case magic[0] == 0x49 && magic[1] == 0x44 && magic[2] == 0x33:
		return mp3Duration(path)
	case string(magic[:4]) == "OggS":
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".opus" {
			return opusDuration(path)
		}
		return oggDuration(path)
	case string(magic[4:8]) == "ftyp":
		// mp4Duration reads boxes from the beginning of the file
		f.Seek(0, io.SeekStart) //nolint:errcheck
		return mp4Duration(f)
	}

	return 0
}

// flacDuration reads total_samples and sample_rate from the STREAMINFO block.
// STREAMINFO is always the first metadata block in a valid FLAC stream.
// Spec: https://xiph.org/flac/format.html#metadata_block_streaminfo
func flacDuration(r io.Reader) int64 {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0
	}
	blockType := hdr[0] & 0x7F
	if blockType != 0 {
		return 0
	}
	blockLen := int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
	if blockLen < 18 {
		return 0
	}
	data := make([]byte, blockLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0
	}
	sampleRate := int64(data[10])<<12 | int64(data[11])<<4 | int64(data[12]>>4)
	totalSamples := int64(data[13]&0x0F)<<32 |
		int64(data[14])<<24 |
		int64(data[15])<<16 |
		int64(data[16])<<8 |
		int64(data[17])
	if sampleRate == 0 {
		return 0
	}
	return totalSamples * 1000 / sampleRate
}

// wavDuration reads the RIFF/WAVE fmt chunk to compute duration from data chunk size.
func wavDuration(r io.Reader) int64 {
	var sampleRate, byteRate uint32
	var blockAlign uint16
	for {
		var id [4]byte
		var size uint32
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			return 0
		}
		if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
			return 0
		}
		switch string(id[:]) {
		case "fmt ":
			var audioFmt, channels uint16
			//nolint:errcheck // partial-read is fine; sampleRate/byteRate may stay zero → handled below
			binary.Read(r, binary.LittleEndian, &audioFmt)
			binary.Read(r, binary.LittleEndian, &channels)   //nolint:errcheck
			binary.Read(r, binary.LittleEndian, &sampleRate) //nolint:errcheck
			binary.Read(r, binary.LittleEndian, &byteRate)   //nolint:errcheck
			binary.Read(r, binary.LittleEndian, &blockAlign) //nolint:errcheck
			if size > 16 {
				io.CopyN(io.Discard, r, int64(size-16)) //nolint:errcheck
			}
		case "data":
			if byteRate > 0 {
				return int64(size) * 1000 / int64(byteRate)
			}
			return 0
		default:
			if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
				return 0
			}
		}
	}
}

// aiffDuration reads the AIFF COMM chunk for sample count and sample rate.
func aiffDuration(r io.Reader) int64 {
	// Skip 4 bytes (size of FORM chunk already read as magic[4:8])
	var formSize uint32
	if err := binary.Read(r, binary.BigEndian, &formSize); err != nil {
		return 0
	}
	var formType [4]byte
	if _, err := io.ReadFull(r, formType[:]); err != nil {
		return 0
	}
	if string(formType[:]) != "AIFF" && string(formType[:]) != "AIFC" {
		return 0
	}
	for {
		var id [4]byte
		var size uint32
		if err := binary.Read(r, binary.BigEndian, &id); err != nil {
			return 0
		}
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return 0
		}
		if string(id[:]) == "COMM" {
			var channels int16
			var numSampleFrames uint32
			var bitsPerSample int16
			var sampleRateBytes [10]byte
			binary.Read(r, binary.BigEndian, &channels)        //nolint:errcheck
			binary.Read(r, binary.BigEndian, &numSampleFrames) //nolint:errcheck
			binary.Read(r, binary.BigEndian, &bitsPerSample)   //nolint:errcheck
			if _, err := io.ReadFull(r, sampleRateBytes[:]); err != nil {
				return 0
			}
			// 80-bit extended float: exponent in [0:2], mantissa in [2:10]
			exp := int(int16(binary.BigEndian.Uint16(sampleRateBytes[0:2]))&0x7FFF) - 16383 //nolint:gosec
			mant := binary.BigEndian.Uint64(sampleRateBytes[2:])
			if exp < 0 || exp > 63 {
				return 0
			}
			sampleRate := int64(mant >> (63 - uint(exp))) //nolint:gosec
			if sampleRate == 0 {
				return 0
			}
			return int64(numSampleFrames) * 1000 / sampleRate
		}
		if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
			return 0
		}
	}
}

// mp3Duration uses file size and the bitrate from the first Xing/VBRI/frame header.
// This is an approximation; accurate for CBR and good for VBR via Xing header.
func mp3Duration(path string) int64 {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return 0
	}

	// Skip ID3v2 tag if present
	var header [10]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return 0
	}
	offset := int64(0)
	if header[0] == 'I' && header[1] == 'D' && header[2] == '3' {
		tagSize := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + tagSize
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return 0
		}
	} else {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0
		}
	}

	// Read enough to find first valid frame header
	buf := make([]byte, 4096)
	n, err2 := f.Read(buf)
	if n == 0 || err2 != nil {
		return 0
	}
	buf = buf[:n]

	for i := 0; i < len(buf)-3; i++ {
		if buf[i] != 0xFF || (buf[i+1]&0xE0) != 0xE0 {
			continue
		}
		b1 := buf[i+1]
		b2 := buf[i+2]

		layer := (b1 >> 1) & 0x03
		bitrateIdx := (b2 >> 4) & 0x0F
		srIdx := (b2 >> 2) & 0x03

		if layer == 0 || bitrateIdx == 0 || bitrateIdx == 15 || srIdx == 3 {
			continue
		}

		mpegVer := (b1 >> 3) & 0x03
		sampleRates := [4][4]int{{11025, 12000, 8000, 0}, {0, 0, 0, 0}, {22050, 24000, 16000, 0}, {44100, 48000, 32000, 0}}
		// layer 3 = MP3, layer 2 = MP2, layer 1 = MP1 (indices inverted: layer field 1=III, 2=II, 3=I)
		var bitrates [4][4][16]int
		bitrates[3][1] = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
		bitrates[3][2] = [16]int{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0}
		bitrates[2][1] = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}

		sr := sampleRates[mpegVer][srIdx]
		br := bitrates[mpegVer][layer][bitrateIdx]
		if sr == 0 || br == 0 {
			continue
		}

		// Check for Xing/Info header (VBR)
		xingOff := i + 4 + 32      // mono: +17, stereo: +32; use stereo as safe default
		if xingOff+11 < len(buf) { //nolint:nestif
			if string(buf[xingOff:xingOff+4]) == "Xing" || string(buf[xingOff:xingOff+4]) == "Info" {
				if buf[xingOff+7]&0x01 != 0 { // frames flag
					frames := int64(buf[xingOff+8])<<24 | int64(buf[xingOff+9])<<16 | int64(buf[xingOff+10])<<8 | int64(buf[xingOff+11])
					samplesPerFrame := int64(1152)
					if layer == 3 {
						samplesPerFrame = 384
					}
					return frames * samplesPerFrame * 1000 / int64(sr)
				}
			}
		}

		// CBR approximation: file size / bitrate
		audioSize := stat.Size() - offset - int64(i)
		return audioSize * 8 / int64(br)
	}
	return 0
}

// mp4Duration finds the mvhd box in an MP4/M4A file for duration and timescale.
func mp4Duration(r io.ReadSeeker) int64 {
	for {
		var size uint32
		var boxType [4]byte
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return 0
		}
		if _, err := io.ReadFull(r, boxType[:]); err != nil {
			return 0
		}
		if size < 8 {
			return 0
		}
		switch string(boxType[:]) {
		case "mvhd":
			var version byte
			if err := binary.Read(r, binary.BigEndian, &version); err != nil {
				return 0
			}
			io.CopyN(io.Discard, r, 3) //nolint:errcheck // flags
			if version == 1 {
				io.CopyN(io.Discard, r, 16) //nolint:errcheck // creation+modification time (8 bytes each)
				var timescale uint32
				var duration uint64
				binary.Read(r, binary.BigEndian, &timescale) //nolint:errcheck
				binary.Read(r, binary.BigEndian, &duration)  //nolint:errcheck
				if timescale == 0 {
					return 0
				}
				return int64(duration) * 1000 / int64(timescale)
			}
			io.CopyN(io.Discard, r, 8) //nolint:errcheck // creation+modification time (4 bytes each)
			var timescale, duration uint32
			binary.Read(r, binary.BigEndian, &timescale) //nolint:errcheck
			binary.Read(r, binary.BigEndian, &duration)  //nolint:errcheck
			if timescale == 0 {
				return 0
			}
			return int64(duration) * 1000 / int64(timescale)
		case "moov", "trak", "mdia", "minf", "stbl":
			// Container boxes — descend into them (don't skip)
			continue
		default:
			if _, err := io.CopyN(io.Discard, r, int64(size)-8); err != nil {
				return 0
			}
		}
	}
}

// oggDuration computes duration for Ogg Vorbis by seeking to the last page.
func oggDuration(path string) int64 {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	var firstPage [58]byte
	if _, err2 := io.ReadFull(f, firstPage[:]); err2 != nil {
		return 0
	}
	sampleRate := oggVorbisSampleRate(f)
	if sampleRate == 0 {
		return 0
	}

	stat, err := f.Stat()
	if err != nil {
		return 0
	}
	// Seek near end of file to find last OGG page granule position
	seekPos := stat.Size() - 65536
	if seekPos < 0 {
		seekPos = 0
	}
	if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
		return 0
	}
	tail := make([]byte, stat.Size()-seekPos)
	if _, err := io.ReadFull(f, tail); err != nil {
		return 0
	}
	// Find last OggS page and read granule position
	lastGranule := int64(-1)
	for i := 0; i < len(tail)-27; i++ {
		if tail[i] == 'O' && tail[i+1] == 'g' && tail[i+2] == 'g' && tail[i+3] == 'S' {
			granule := int64(binary.LittleEndian.Uint64(tail[i+6:])) //nolint:gosec
			if granule > lastGranule {
				lastGranule = granule
			}
		}
	}
	if lastGranule <= 0 {
		return 0
	}
	return lastGranule * 1000 / sampleRate
}

func oggVorbisSampleRate(f *os.File) int64 {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0
	}
	buf := make([]byte, 4096)
	n, readErr := f.Read(buf)
	if n == 0 || readErr != nil {
		return 0
	}
	for i := 0; i < n-10; i++ {
		if buf[i] == 0x01 && string(buf[i+1:i+7]) == "vorbis" {
			return int64(binary.LittleEndian.Uint32(buf[i+11:]))
		}
	}
	return 0
}

// opusDuration computes duration for Ogg Opus by seeking to the last page.
// Opus always uses a 48kHz granule position.
func opusDuration(path string) int64 {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	var preskip uint16
	buf := make([]byte, 512)
	n, readErr := f.Read(buf)
	if readErr != nil {
		n = 0
	}
	for i := 0; i < n-18; i++ {
		if string(buf[i:i+8]) == "OpusHead" {
			preskip = binary.LittleEndian.Uint16(buf[i+10:])
			break
		}
	}

	stat, err := f.Stat()
	if err != nil {
		return 0
	}
	seekPos := stat.Size() - 65536
	if seekPos < 0 {
		seekPos = 0
	}
	if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
		return 0
	}
	tail := make([]byte, stat.Size()-seekPos)
	if _, err := io.ReadFull(f, tail); err != nil {
		return 0
	}
	lastGranule := int64(-1)
	for i := 0; i < len(tail)-27; i++ {
		if tail[i] == 'O' && tail[i+1] == 'g' && tail[i+2] == 'g' && tail[i+3] == 'S' {
			granule := int64(binary.LittleEndian.Uint64(tail[i+6:])) //nolint:gosec
			if granule > lastGranule {
				lastGranule = granule
			}
		}
	}
	if lastGranule <= 0 {
		return 0
	}
	const opusSampleRate = 48000
	return (lastGranule - int64(preskip)) * 1000 / opusSampleRate
}
