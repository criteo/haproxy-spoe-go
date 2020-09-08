package spoe

type spoeError int

const (
	spoeErrorNone spoeError = iota
	spoeErrorIO
	spoeErrorTimeout
	spoeErrorTooBig
	spoeErrorInvalid
	spoeErrorNoVSN
	spoeErrorNoFrameSize
	spoeErrorNoCap
	spoeErrorBadVsn
	spoeErrorBadFrameSize
	spoeErrorFragNotSupported
	spoeErrorInterlacedFrames
	spoeErrorFrameIDNotfound
	spoeErrorRes
	spoeErrorUnknown spoeError = 99
)

var spoeErrorMessages = map[spoeError]string{
	spoeErrorNone:             "normal",
	spoeErrorIO:               "I/O error",
	spoeErrorTimeout:          "a timeout occurred",
	spoeErrorTooBig:           "frame is too big",
	spoeErrorInvalid:          "invalid frame received",
	spoeErrorNoVSN:            "version value not found",
	spoeErrorNoFrameSize:      "max-frame-size value not found",
	spoeErrorNoCap:            "capabilities value not found",
	spoeErrorBadVsn:           "unsupported version",
	spoeErrorBadFrameSize:     "max-frame-size too big or too small",
	spoeErrorFragNotSupported: "fragmentation not supported",
	spoeErrorInterlacedFrames: "invalid interlaced frames",
	spoeErrorFrameIDNotfound:  "frame-id not found",
	spoeErrorRes:              "resource allocation error",
	spoeErrorUnknown:          "an unknown error occurred",
}
