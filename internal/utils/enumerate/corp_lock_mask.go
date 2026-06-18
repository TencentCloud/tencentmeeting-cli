package enumerate

// CorpLockMask represents the bit mask of enterprise-locked meeting settings.
// Each bit indicates that the corresponding setting is enforced (locked) by the enterprise.
type CorpLockMask uint32

const (
	CorpLockMaskWaterMarkType  CorpLockMask = 0x1 // bit0: 文字水印被企业锁定
	CorpLockMaskAudioWatermark CorpLockMask = 0x2 // bit1: 音频水印被企业锁定
	CorpLockMaskAutoRecordType CorpLockMask = 0x4 // bit2: 自动录制被企业锁定
	CorpLockMaskAutoAsr        CorpLockMask = 0x8 // bit3: 自动语音识别被企业锁定
)

// corpLockMaskOrder defines the iteration order used when expanding a mask value.
var corpLockMaskOrder = []CorpLockMask{
	CorpLockMaskWaterMarkType,
	CorpLockMaskAudioWatermark,
	CorpLockMaskAutoRecordType,
	CorpLockMaskAutoAsr,
}

var corpLockMaskNames = map[CorpLockMask]string{
	CorpLockMaskWaterMarkType:  "Text Watermark",
	CorpLockMaskAudioWatermark: "Audio Watermark",
	CorpLockMaskAutoRecordType: "Auto Recording",
	CorpLockMaskAutoAsr:        "Auto Speech Recognition",
}

// CorpLockMaskName returns the locked-setting label for a SINGLE bit value,
// or "Unknown" for unrecognized bits. For combined mask values (multiple bits set),
// use CorpLockMaskNames instead.
func CorpLockMaskName(bit uint32) string {
	if name, ok := corpLockMaskNames[CorpLockMask(bit)]; ok {
		return name
	}
	return "Unknown"
}

// CorpLockMaskNames returns the locked-setting labels matched by the given mask value,
// preserving the canonical bit order. Unknown bits are ignored.
func CorpLockMaskNames(mask uint32) []string {
	if mask == 0 {
		return nil
	}
	var result []string
	for _, bit := range corpLockMaskOrder {
		if uint32(bit)&mask != 0 {
			result = append(result, corpLockMaskNames[bit])
		}
	}
	return result
}
