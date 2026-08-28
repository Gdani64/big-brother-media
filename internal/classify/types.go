package classify

type MediaType int

const (
	TypeMovie MediaType = iota
	TypeTVShow
	TypeAudioBook
	TypeBook
	TypeMusic
	TypeSoftware
	TypeGame
	TypeUnknown
)

var typeName = map[MediaType]string{
	TypeMovie:     "movie",
	TypeTVShow:    "tv_show",
	TypeAudioBook: "audio_book",
	TypeBook:      "book",
	TypeMusic:     "music",
	TypeSoftware:  "software",
	TypeGame:      "game",
	TypeUnknown:   "unknown",
}

func (mt MediaType) String() string {
	return typeName[mt]
}

// AllMediaTypeNames returns every valid MediaType name, e.g. for use as an
// enum in an external schema (LLM structured output, CLI flags, etc).
func AllMediaTypeNames() []string {
	names := make([]string, 0, len(typeName))
	for _, n := range typeName {
		names = append(names, n)
	}
	return names
}

// MediaTypeFromName is the inverse of MediaType.String, returning false if
// name doesn't match a known MediaType.
func MediaTypeFromName(name string) (MediaType, bool) {
	for mt, n := range typeName {
		if n == name {
			return mt, true
		}
	}
	return TypeUnknown, false
}
