package zip

const (
	SigCDHeader       = uint32(0x02_01_4B_50)
	SigLocalHeader    = uint32(0x04_03_4B_50)
	SigEOCD           = uint32(0x06_05_4B_50)
	SigFileDescriptor = uint32(0x08_07_4B_50)

	VerRequired = uint16(20) // zip 2.0
	VerMadeBy   = uint16(20)

	FlagUTF8Filename   = uint16(1 << 11)
	FlagInfoComesLater = uint16(1 << 3)

	MethodStore = uint16(0)
)
