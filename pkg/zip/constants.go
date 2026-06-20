package zip

const (
	SigCDHeader         = uint32(0x02_01_4B_50)
	SigLocalHeader      = uint32(0x04_03_4B_50)
	SigEOCD             = uint32(0x06_05_4B_50)
	SigZip64EOCD        = uint32(0x06_06_4B_50)
	SigZip64EOCDLocator = uint32(0x07_06_4B_50)
	SigFileDescriptor   = uint32(0x08_07_4B_50)

	ZipVer20 = uint16(20)
	ZipVer45 = uint16(45)

	Zip64ExtraFieldID   = uint16(0x00_01)
	ExtraFieldSize      = uint16(24)
	Zip64ExtraFieldSize = uint16(28)

	LocalExtraFieldSize      = uint16(20)
	LocalZip64ExtraFieldSize = uint16(16)

	FlagUTF8Filename   = uint16(1 << 11)
	FlagInfoComesLater = uint16(1 << 3)

	MethodStore   = uint16(0)
	MethodDeflate = uint16(8)

	Max32 = uint32(0xFFFFFFFF)
	Max16 = uint16(0xFFFF)
)
