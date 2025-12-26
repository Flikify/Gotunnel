package wasm

import (
	"github.com/tetratelabs/wazero/api"
)

// ReadString 从 WASM 内存读取字符串
func ReadString(mem api.Memory, ptr, len uint32) (string, bool) {
	data, ok := mem.Read(ptr, len)
	if !ok {
		return "", false
	}
	return string(data), true
}

// WriteString 向 WASM 内存写入字符串
func WriteString(mem api.Memory, ptr uint32, s string) bool {
	return mem.Write(ptr, []byte(s))
}

// ReadBytes 从 WASM 内存读取字节
func ReadBytes(mem api.Memory, ptr, len uint32) ([]byte, bool) {
	return mem.Read(ptr, len)
}

// WriteBytes 向 WASM 内存写入字节
func WriteBytes(mem api.Memory, ptr uint32, data []byte) bool {
	return mem.Write(ptr, data)
}
