package service

import (
	"encoding/binary"
	"io"
)

// readMP4BoxSizeAndType 读取 MP4 顶层 box 的 size 与 type。
// - size == 1：表示扩展 size 存在于 offset+8 处的 8 字节
// - size == 0：表示 box 一直延伸到文件末尾
func readMP4BoxSizeAndType(reader io.ReaderAt, offset int64, fileSize int64) (boxSize int64, boxType string, err error) {
	// box header: size(4) + type(4)
	header := make([]byte, 8)
	if _, err = reader.ReadAt(header, offset); err != nil {
		return 0, "", err
	}

	size32 := binary.BigEndian.Uint32(header[0:4])
	boxType = string(header[4:8])

	switch size32 {
	case 1:
		extHeader := make([]byte, 8)
		if _, err = reader.ReadAt(extHeader, offset+8); err != nil {
			return 0, "", err
		}
		boxSize = int64(binary.BigEndian.Uint64(extHeader))
	case 0:
		boxSize = fileSize - offset
	default:
		boxSize = int64(size32)
	}

	return boxSize, boxType, nil
}

