// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package mc

import (
	"bytes"
	"math"
)

// Tag identifiers of the NBT subset a Minecraft world save needs.
const (
	tagEnd byte = iota
	tagByte
	tagShort
	tagInt
	tagLong
	tagFloat
	tagDouble
	tagByteArray
	tagString
	tagList
	tagCompound
	tagIntArray
	tagLongArray
)

// nbt writes big-endian uncompressed NBT. Compression is applied by the caller
// because chunks use zlib while level.dat uses gzip.
type nbt struct {
	buf bytes.Buffer
}

func (n *nbt) bytes() []byte {
	return n.buf.Bytes()
}

func (n *nbt) u16(value uint16) {
	n.buf.WriteByte(byte(value >> 8))
	n.buf.WriteByte(byte(value))
}

func (n *nbt) u32(value uint32) {
	n.buf.WriteByte(byte(value >> 24))
	n.buf.WriteByte(byte(value >> 16))
	n.buf.WriteByte(byte(value >> 8))
	n.buf.WriteByte(byte(value))
}

func (n *nbt) u64(value uint64) {
	n.u32(uint32(value >> 32))
	n.u32(uint32(value))
}

func (n *nbt) text(value string) {
	n.u16(uint16(len(value)))
	n.buf.WriteString(value)
}

func (n *nbt) key(id byte, name string) {
	n.buf.WriteByte(id)
	n.text(name)
}

func (n *nbt) byteField(name string, value int8) {
	n.key(tagByte, name)
	n.buf.WriteByte(byte(value))
}

func (n *nbt) intField(name string, value int32) {
	n.key(tagInt, name)
	n.u32(uint32(value))
}

func (n *nbt) longField(name string, value int64) {
	n.key(tagLong, name)
	n.u64(uint64(value))
}

func (n *nbt) floatField(name string, value float32) {
	n.key(tagFloat, name)
	n.u32(math.Float32bits(value))
}

func (n *nbt) doubleField(name string, value float64) {
	n.key(tagDouble, name)
	n.u64(math.Float64bits(value))
}

func (n *nbt) stringField(name, value string) {
	n.key(tagString, name)
	n.text(value)
}

func (n *nbt) longArrayField(name string, values []int64) {
	n.key(tagLongArray, name)
	n.u32(uint32(len(values)))
	for _, value := range values {
		n.u64(uint64(value))
	}
}

// compound writes a named compound. Pass an empty name for the NBT root.
func (n *nbt) compound(name string, body func()) {
	n.key(tagCompound, name)
	body()
	n.buf.WriteByte(tagEnd)
}

// list writes a named list. An empty list is written with element type End,
// which is what vanilla Minecraft produces.
func (n *nbt) list(name string, element byte, length int, body func()) {
	n.key(tagList, name)
	if length == 0 {
		element = tagEnd
	}
	n.buf.WriteByte(element)
	n.u32(uint32(length))
	if body != nil {
		body()
	}
}

// element writes one anonymous compound inside a list.
func (n *nbt) element(body func()) {
	body()
	n.buf.WriteByte(tagEnd)
}
