package enumerate

import (
	"reflect"
	"testing"
)

func TestCorpLockMaskName(t *testing.T) {
	tests := []struct {
		bit  uint32
		want string
	}{
		{0x1, "Text Watermark"},
		{0x2, "Audio Watermark"},
		{0x4, "Auto Recording"},
		{0x8, "Auto Speech Recognition"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := CorpLockMaskName(tt.bit)
			if got != tt.want {
				t.Errorf("CorpLockMaskName(%#x) = %q, want %q", tt.bit, got, tt.want)
			}
		})
	}
}

func TestCorpLockMaskName_Unknown(t *testing.T) {
	unknownBits := []uint32{0x0, 0x10, 0x20, 0x100, 0x3}
	for _, b := range unknownBits {
		t.Run("unknown", func(t *testing.T) {
			got := CorpLockMaskName(b)
			if got != "Unknown" {
				t.Errorf("CorpLockMaskName(%#x) = %q, want %q", b, got, "Unknown")
			}
		})
	}
}

func TestCorpLockMaskNames(t *testing.T) {
	tests := []struct {
		name string
		mask uint32
		want []string
	}{
		{"empty", 0x0, nil},
		{"single_text_watermark", 0x1, []string{"Text Watermark"}},
		{"single_audio_watermark", 0x2, []string{"Audio Watermark"}},
		{"single_auto_record", 0x4, []string{"Auto Recording"}},
		{"single_auto_asr", 0x8, []string{"Auto Speech Recognition"}},
		{"all_bits", 0xF, []string{"Text Watermark", "Audio Watermark", "Auto Recording", "Auto Speech Recognition"}},
		{"text_and_asr", 0x9, []string{"Text Watermark", "Auto Speech Recognition"}},
		{"with_unknown_bits", 0x11, []string{"Text Watermark"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CorpLockMaskNames(tt.mask)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CorpLockMaskNames(%#x) = %v, want %v", tt.mask, got, tt.want)
			}
		})
	}
}
