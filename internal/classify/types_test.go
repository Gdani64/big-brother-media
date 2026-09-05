package classify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMediaTypeFromName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  MediaType
		found bool
	}{
		{
			name:  "Type movie",
			input: "movie",
			want:  TypeMovie,
			found: true,
		},
		{
			name:  "Type tv_show",
			input: "tv_show",
			want:  TypeTVShow,
			found: true,
		},
		{
			name:  "Type audio_book",
			input: "audio_book",
			want:  TypeAudioBook,
			found: true,
		},
		{
			name:  "Type book",
			input: "book",
			want:  TypeBook,
			found: true,
		},
		{
			name:  "Type music",
			input: "music",
			want:  TypeMusic,
			found: true,
		},
		{
			name:  "Type software",
			input: "software",
			want:  TypeSoftware,
			found: true,
		},
		{
			name:  "Type game",
			input: "game",
			want:  TypeGame,
			found: true,
		},
		{
			name:  "Type unknown",
			input: "unknown",
			want:  TypeUnknown,
			found: true,
		},
		{
			name:  "unrecognized name returns false",
			input: "not_a_real_type",
			want:  TypeUnknown,
			found: false,
		},
		{
			name:  "empty string returns false",
			input: "",
			want:  TypeUnknown,
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaType, found := MediaTypeFromName(tt.input)
			require.True(t, found == tt.found)
			require.Equal(t, tt.want, mediaType)
		})
	}
}

func TestAllMediaTypeNames(t *testing.T) {
	res := AllMediaTypeNames()
	require.ElementsMatch(t, res, me)
}
