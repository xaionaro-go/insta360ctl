package camera

import "strings"

// Model identifies an Insta360 camera model.
type Model int

const (
	ModelUnknown Model = iota
	ModelOneX
	ModelOneX2
	ModelOneX3
	ModelOneX4
	ModelX3
	ModelX4
	ModelX5
	ModelOneRS
	ModelOneR
	ModelGo3
	ModelGo3S
	ModelAcePro
	ModelAce
	modelEnd
)

func (m Model) String() string {
	switch m {
	case ModelOneX:
		return "ONE X"
	case ModelOneX2:
		return "ONE X2"
	case ModelOneX3:
		return "ONE X3"
	case ModelOneX4:
		return "ONE X4"
	case ModelX3:
		return "X3"
	case ModelX4:
		return "X4"
	case ModelX5:
		return "X5"
	case ModelOneRS:
		return "ONE RS"
	case ModelOneR:
		return "ONE R"
	case ModelGo3:
		return "GO 3"
	case ModelGo3S:
		return "GO 3S"
	case ModelAcePro:
		return "Ace Pro"
	case ModelAce:
		return "Ace"
	default:
		return "Unknown"
	}
}

// bleNamePrefixes maps BLE advertised name prefixes to camera models.
var bleNamePrefixes = []struct {
	prefix string
	model  Model
}{
	// Longer prefixes first to avoid false matches.
	{"ONE X3 ", ModelOneX3},
	{"ONE X2 ", ModelOneX2},
	{"ONE X4 ", ModelOneX4},
	{"ONE RS ", ModelOneRS},
	{"ONE R ", ModelOneR},
	{"ONE X ", ModelOneX},
	{"ONE ", ModelOneX}, // fallback for "ONE XXXX" without specific model
	{"X5 ", ModelX5},
	{"X4 ", ModelX4},
	{"X3 ", ModelX3},
	{"GO 3S ", ModelGo3S},
	{"GO 3 ", ModelGo3},
	{"Ace Pro ", ModelAcePro},
	{"Ace ", ModelAce},
}

// IdentifyFromBLEName identifies the camera model from its BLE advertised name.
// Insta360 cameras typically advertise names like "X3 ABCDEF", "X4 123456", etc.
func IdentifyFromBLEName(name string) Model {
	for _, p := range bleNamePrefixes {
		if strings.HasPrefix(name, p.prefix) {
			return p.model
		}
	}
	return ModelUnknown
}

// IsInsta360Device returns true if the BLE name looks like an Insta360 camera.
func IsInsta360Device(name string) bool {
	return IdentifyFromBLEName(name) != ModelUnknown
}

// SupportsDirectControl returns whether this model supports Architecture B
// (direct BLE control via BE80 service).
func (m Model) SupportsDirectControl() bool {
	switch m {
	case ModelX3, ModelX4, ModelX5, ModelOneX3, ModelOneX4, ModelOneRS, ModelAcePro, ModelAce,
		ModelGo3, ModelGo3S:
		return true
	default:
		return false
	}
}

// ProtoFormat identifies the wire protocol format for Architecture B.
type ProtoFormat int

const (
	// ProtoFormatHeader16 is the 16-byte header format used by X3, ONE R, ONE RS.
	ProtoFormatHeader16 ProtoFormat = iota
	// ProtoFormatFFFrame is the FF-framed format with CRC-16/Modbus used by GO 2, GO 3.
	ProtoFormatFFFrame
)

// DirectProtoFormat returns which wire protocol format this camera model uses
// for Architecture B (BE80/BE81/BE82) communication.
func (m Model) DirectProtoFormat() ProtoFormat {
	switch m {
	case ModelGo3, ModelGo3S:
		return ProtoFormatFFFrame
	default:
		return ProtoFormatHeader16
	}
}

// SupportsRemoteControl returns whether this model supports Architecture A
// (GPS Remote emulation via CE80 service).
func (m Model) SupportsRemoteControl() bool {
	switch m {
	case ModelOneX, ModelOneX2, ModelOneX3, ModelOneX4, ModelX3, ModelX4, ModelX5, ModelOneRS, ModelOneR:
		return true
	default:
		return false
	}
}
