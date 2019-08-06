package types

const BacnetMaxInstance = 0x3FFFFF
const BacnetInstanceBits = 0x16
const BacnetMaxObject = 0x3FF
const BacnetArrayAll = 0xFFFFFFFF

const MaxMacLen = 7
const MaxNpdu = 1 + 1 + 2 + 1 + MaxMacLen + 2 + 1 + MaxMacLen + 1 + 1 + 2
const MaxApdu = 1476
const MaxPdu = MaxApdu + MaxNpdu
const MaxHeader = 1 + 1 + 2
const MaxMpdu = MaxHeader + MaxPdu
const MaxBitstringBytes = 15
const MaxCharacterStringBytes = MaxApdu - 6
const MaxOctetStringBytes = MaxApdu - 6

const BACnetProtocol = 0x81
const BACnetVersion = 0x01
