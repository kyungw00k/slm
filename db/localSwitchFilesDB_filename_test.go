package db

import (
	"testing"
)

func TestParseTitleNameFromFileName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "Korean locale suffix",
			filename: "A_Street_Cats_Tale_NekoNeko_Edition__Kor_.nsp",
			expected: "A Street Cats Tale NekoNeko Edition",
		},
		{
			name:     "Japanese locale suffix",
			filename: "Platform_8__Jpn_.nsp",
			expected: "Platform 8",
		},
		{
			name:     "English locale suffix",
			filename: "Super_Mario_Odyssey__Eng_.nsp",
			expected: "Super Mario Odyssey",
		},
		{
			name:     "Title ID and version in brackets",
			filename: "The_Legend_of_Zelda_[01006DD01EB6C000][v0].nsp",
			expected: "The Legend of Zelda",
		},
		{
			name:     "Only title ID and version (no extractable name)",
			filename: "[01006DD01EB6C000][v0].nsp",
			expected: "",
		},
		{
			name:     "Complex pattern with locale and brackets",
			filename: "Animal_Crossing_New_Horizons__Kor_[0100ABF008968000][v196608].nsp",
			expected: "Animal Crossing New Horizons",
		},
		{
			name:     "NSZ file extension",
			filename: "Pokemon_Scarlet__Jpn_.nsz",
			expected: "Pokemon Scarlet",
		},
		{
			name:     "XCI file extension",
			filename: "Mario_Kart_8_Deluxe__Eng_.xci",
			expected: "Mario Kart 8 Deluxe",
		},
		{
			name:     "Multiple underscores in name",
			filename: "The_Witcher_3_Wild_Hunt__Kor_.nsp",
			expected: "The Witcher 3 Wild Hunt",
		},
		{
			name:     "Name with numbers",
			filename: "Persona_5_Royal__Jpn_.nsp",
			expected: "Persona 5 Royal",
		},
		{
			name:     "Short name",
			filename: "Hades__Eng_.nsp",
			expected: "Hades",
		},
		{
			name:     "Name with special edition marker",
			filename: "Dark_Souls_Remastered__Kor_[0100897010F10000][v65536].nsp",
			expected: "Dark Souls Remastered",
		},
		{
			name:     "Version without v prefix",
			filename: "Game_Name_[01006DD01EB6C000][0].nsp",
			expected: "Game Name",
		},
		{
			name:     "No locale suffix",
			filename: "Simple_Game_Name.nsp",
			expected: "Simple Game Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTitleNameFromFileName(tt.filename)
			if result != tt.expected {
				t.Errorf("ParseTitleNameFromFileName(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func BenchmarkParseTitleNameFromFileName(b *testing.B) {
	filenames := []string{
		"A_Street_Cats_Tale_NekoNeko_Edition__Kor_.nsp",
		"Platform_8__Jpn_.nsp",
		"The_Legend_of_Zelda_[01006DD01EB6C000][v0].nsp",
		"[01006DD01EB6C000][v0].nsp",
		"Animal_Crossing_New_Horizons__Kor_[0100ABF008968000][v196608].nsp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, filename := range filenames {
			ParseTitleNameFromFileName(filename)
		}
	}
}
