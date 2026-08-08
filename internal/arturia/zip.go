package arturia

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"path"
	"strings"
)

type archiveEntry struct {
	Name string
	Data []byte
}

// buildStoredZip creates the conservative ZIP layout used by the source project:
// no directory entries, no compression, FAT creator headers, deterministic timestamps.
func buildStoredZip(entries []archiveEntry) ([]byte, error) {
	if len(entries) == 0 || len(entries) > 65535 {
		return nil, fmt.Errorf("invalid archive entry count %d", len(entries))
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		clean := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
		if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return nil, fmt.Errorf("unsafe archive path %q", entry.Name)
		}
		if len([]byte(clean)) > 65535 {
			return nil, fmt.Errorf("archive path is too long: %q", clean)
		}
		if seen[clean] {
			return nil, fmt.Errorf("duplicate archive path %q", clean)
		}
		seen[clean] = true
	}

	var buf bytes.Buffer
	offsets := make([]uint32, len(entries))
	crcs := make([]uint32, len(entries))

	for i, entry := range entries {
		name := []byte(path.Clean(strings.ReplaceAll(entry.Name, "\\", "/")))
		if buf.Len() > int(^uint32(0)) {
			return nil, fmt.Errorf("archive exceeds ZIP32 limits")
		}
		offsets[i] = uint32(buf.Len())
		crcs[i] = crc32.ChecksumIEEE(entry.Data)
		buf.Write([]byte{0x50, 0x4b, 0x03, 0x04})
		writeLE(&buf, uint16(20))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, crcs[i])
		writeLE(&buf, uint32(len(entry.Data)))
		writeLE(&buf, uint32(len(entry.Data)))
		writeLE(&buf, uint16(len(name)))
		writeLE(&buf, uint16(0))
		buf.Write(name)
		buf.Write(entry.Data)
	}

	cdStart := buf.Len()
	for i, entry := range entries {
		name := []byte(path.Clean(strings.ReplaceAll(entry.Name, "\\", "/")))
		buf.Write([]byte{0x50, 0x4b, 0x01, 0x02})
		writeLE(&buf, uint16(20)) // FAT creator, ZIP 2.0
		writeLE(&buf, uint16(20))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, crcs[i])
		writeLE(&buf, uint32(len(entry.Data)))
		writeLE(&buf, uint32(len(entry.Data)))
		writeLE(&buf, uint16(len(name)))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint16(0))
		writeLE(&buf, uint32(0))
		writeLE(&buf, offsets[i])
		buf.Write(name)
	}
	cdSize := buf.Len() - cdStart
	buf.Write([]byte{0x50, 0x4b, 0x05, 0x06})
	writeLE(&buf, uint16(0))
	writeLE(&buf, uint16(0))
	writeLE(&buf, uint16(len(entries)))
	writeLE(&buf, uint16(len(entries)))
	writeLE(&buf, uint32(cdSize))
	writeLE(&buf, uint32(cdStart))
	writeLE(&buf, uint16(0))
	return buf.Bytes(), nil
}

func writeLE(buf *bytes.Buffer, value any) {
	_ = binary.Write(buf, binary.LittleEndian, value)
}
