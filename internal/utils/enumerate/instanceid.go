package enumerate

// InstanceType represents the user's terminal device type.
type InstanceType int

const (
	InstanceTypePSTN                      InstanceType = 0  // PSTN
	InstanceTypePC                        InstanceType = 1  // PC
	InstanceTypeMac                       InstanceType = 2  // Mac
	InstanceTypeAndroid                   InstanceType = 3  // Android
	InstanceTypeIOS                       InstanceType = 4  // iOS
	InstanceTypeWeb                       InstanceType = 5  // Web
	InstanceTypeIPad                      InstanceType = 6  // iPad
	InstanceTypeAndroidPad                InstanceType = 7  // Android Pad
	InstanceTypeMiniProgram               InstanceType = 8  // 小程序
	InstanceTypeVoIPSIP                   InstanceType = 9  // voip、sip 设备
	InstanceTypeLinux                     InstanceType = 10 // Linux
	InstanceTypeVisionPro                 InstanceType = 12 // Vision Pro
	InstanceTypeRoomsForTouchWindows      InstanceType = 20 // Rooms for Touch Windows
	InstanceTypeRoomsForTouchMacOS        InstanceType = 21 // Rooms for Touch macOS
	InstanceTypeRoomsForTouchAndroid      InstanceType = 22 // Rooms for Touch Android
	InstanceTypeControllerForTouchWin     InstanceType = 30 // Controller for Touch Windows
	InstanceTypeControllerForTouchAndroid InstanceType = 32 // Controller for Touch Android
	InstanceTypeControllerForTouchIOS     InstanceType = 33 // Controller for Touch iOS
	InstanceTypeHarmonyPhone              InstanceType = 81 // HarmonyOS Phone
	InstanceTypeHarmonyTablet             InstanceType = 82 // HarmonyOS Tablet
	InstanceTypeHarmonyPC                 InstanceType = 83 // HarmonyOS PC
	InstanceTypeHarmonyCockpit            InstanceType = 84 // HarmonyOS Intelligent Cockpit
	InstanceTypeHarmonyARVR               InstanceType = 86 // HarmonyOS AR/VR
)

var instanceTypeNames = map[InstanceType]string{
	InstanceTypePSTN:                      "PSTN",
	InstanceTypePC:                        "PC",
	InstanceTypeMac:                       "Mac",
	InstanceTypeAndroid:                   "Android",
	InstanceTypeIOS:                       "iOS",
	InstanceTypeWeb:                       "Web",
	InstanceTypeIPad:                      "iPad",
	InstanceTypeAndroidPad:                "Android Pad",
	InstanceTypeMiniProgram:               "小程序",
	InstanceTypeVoIPSIP:                   "voip/sip 设备",
	InstanceTypeLinux:                     "Linux",
	InstanceTypeVisionPro:                 "Vision Pro",
	InstanceTypeRoomsForTouchWindows:      "Rooms for Touch Windows",
	InstanceTypeRoomsForTouchMacOS:        "Rooms for Touch macOS",
	InstanceTypeRoomsForTouchAndroid:      "Rooms for Touch Android",
	InstanceTypeControllerForTouchWin:     "Controller for Touch Windows",
	InstanceTypeControllerForTouchAndroid: "Controller for Touch Android",
	InstanceTypeControllerForTouchIOS:     "Controller for Touch iOS",
	InstanceTypeHarmonyPhone:              "HarmonyOS Phone",
	InstanceTypeHarmonyTablet:             "HarmonyOS Tablet",
	InstanceTypeHarmonyPC:                 "HarmonyOS PC",
	InstanceTypeHarmonyCockpit:            "HarmonyOS Intelligent Cockpit",
	InstanceTypeHarmonyARVR:               "HarmonyOS AR/VR",
}

// InstanceTypeName returns the device type name for the given instanceid, or "Unknown" for unrecognized types.
func InstanceTypeName(id int) string {
	if name, ok := instanceTypeNames[InstanceType(id)]; ok {
		return name
	}
	return "Unknown"
}
