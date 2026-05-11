package main

import (
	"os"
	"path/filepath"
	"strings"

	id3 "github.com/bogem/id3v2/v2"
	flac "github.com/go-flac/go-flac"
	flacvorbis "github.com/go-flac/flacvorbis"
)

// tagData holds the six standard fields shared across audio formats.
type tagData struct {
	Title  string
	Artist string
	Album  string
	Year   string
	Track  string
	Genre  string
}

// readTags reads tag metadata from an audio file, dispatching by extension.
func readTags(path string) (tagData, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		return readFLACTags(path)
	default:
		return readMP3Tags(path)
	}
}

// writeTags writes tag metadata to an audio file.
// The write mask controls which of the 6 fields are written:
// [0]=Title, [1]=Artist, [2]=Album, [3]=Year, [4]=Track, [5]=Genre.
func writeTags(path string, data tagData, write [6]bool) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		return writeFLACTags(path, data, write)
	default:
		return writeMP3Tags(path, data, write)
	}
}

// readTagSummary returns the artist and title for a file, used by the browser.
func readTagSummary(path string) tagSummary {
	data, err := readTags(path)
	if err != nil {
		return tagSummary{}
	}
	return tagSummary{artist: data.Artist, title: data.Title}
}

// readMP3Tags reads ID3 tags from an MP3 (or generic ID3-tagged) file.
func readMP3Tags(path string) (tagData, error) {
	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		return tagData{}, err
	}
	defer tag.Close()

	var track string
	if frame := tag.GetLastFrame("TRCK"); frame != nil {
		if tf, ok := frame.(id3.TextFrame); ok {
			track = tf.Text
		}
	}

	return tagData{
		Title:  tag.Title(),
		Artist: tag.Artist(),
		Album:  tag.Album(),
		Year:   tag.Year(),
		Track:  track,
		Genre:  tag.Genre(),
	}, nil
}

// writeMP3Tags writes ID3 tags to an MP3 (or generic ID3-tagged) file.
func writeMP3Tags(path string, data tagData, write [6]bool) error {
	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		return err
	}
	defer tag.Close()

	if write[0] {
		tag.SetTitle(data.Title)
	}
	if write[1] {
		tag.SetArtist(data.Artist)
	}
	if write[2] {
		tag.SetAlbum(data.Album)
	}
	if write[3] {
		tag.SetYear(data.Year)
	}
	if write[4] {
		tag.DeleteFrames("TRCK")
		if data.Track != "" {
			tag.AddTextFrame("TRCK", id3.EncodingUTF8, data.Track)
		}
	}
	if write[5] {
		tag.SetGenre(data.Genre)
	}

	return tag.Save()
}

// readFLACTags reads Vorbis Comment tags from a FLAC file.
// Uses ParseMetadata to avoid reading audio frames.
func readFLACTags(path string) (tagData, error) {
	f, err := os.Open(path)
	if err != nil {
		return tagData{}, err
	}
	defer f.Close()

	meta, err := flac.ParseMetadata(f)
	if err != nil {
		return tagData{}, err
	}

	block := findVorbisCommentBlock(meta)
	if block == nil {
		return tagData{}, nil
	}

	cmt, err := flacvorbis.ParseFromMetaDataBlock(*block)
	if err != nil || cmt == nil {
		return tagData{}, nil
	}

	get := func(key string) string {
		vals, err := cmt.Get(key)
		if err != nil || len(vals) == 0 {
			return ""
		}
		return vals[0]
	}

	return tagData{
		Title:  get(flacvorbis.FIELD_TITLE),
		Artist: get(flacvorbis.FIELD_ARTIST),
		Album:  get(flacvorbis.FIELD_ALBUM),
		Year:   get(flacvorbis.FIELD_DATE),
		Track:  get(flacvorbis.FIELD_TRACKNUMBER),
		Genre:  get(flacvorbis.FIELD_GENRE),
	}, nil
}

// writeFLACTags writes Vorbis Comment tags to a FLAC file.
func writeFLACTags(path string, data tagData, write [6]bool) error {
	file, err := flac.ParseFile(path)
	if err != nil {
		return err
	}

	// Find or build the Vorbis Comment block.
	var cmt *flacvorbis.MetaDataBlockVorbisComment
	idx := findVorbisCommentIndex(file)
	if idx >= 0 {
		cmt, err = flacvorbis.ParseFromMetaDataBlock(*file.Meta[idx])
		if err != nil {
			cmt = flacvorbis.New()
		}
	} else {
		cmt = flacvorbis.New()
	}

	type fieldMapping struct {
		key string
		val string
		set bool
	}
	fields := []fieldMapping{
		{flacvorbis.FIELD_TITLE, data.Title, write[0]},
		{flacvorbis.FIELD_ARTIST, data.Artist, write[1]},
		{flacvorbis.FIELD_ALBUM, data.Album, write[2]},
		{flacvorbis.FIELD_DATE, data.Year, write[3]},
		{flacvorbis.FIELD_TRACKNUMBER, data.Track, write[4]},
		{flacvorbis.FIELD_GENRE, data.Genre, write[5]},
	}

	for _, fm := range fields {
		if !fm.set {
			continue
		}
		// Remove existing values for this key (Vorbis Comments can have duplicates).
		filtered := cmt.Comments[:0]
		for _, c := range cmt.Comments {
			parts := strings.SplitN(c, "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], fm.key) {
				continue
			}
			filtered = append(filtered, c)
		}
		cmt.Comments = filtered
		if fm.val != "" {
			if err := cmt.Add(fm.key, fm.val); err != nil {
				return err
			}
		}
	}

	block := cmt.Marshal()
	if idx >= 0 {
		file.Meta[idx] = &block
	} else {
		file.Meta = append(file.Meta, &block)
	}

	return file.Save(path)
}

// findVorbisCommentBlock returns a pointer to the first Vorbis Comment block.
func findVorbisCommentBlock(f *flac.File) *flac.MetaDataBlock {
	for _, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			return m
		}
	}
	return nil
}

// findVorbisCommentIndex returns the index of the first Vorbis Comment block, or -1.
func findVorbisCommentIndex(f *flac.File) int {
	for i, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			return i
		}
	}
	return -1
}
