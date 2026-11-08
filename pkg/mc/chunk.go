// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package mc

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// compressChunk serializes one chunk to zlib compressed NBT. It returns nil
// when every section of the chunk is empty.
func compressChunk(key [2]int32, c *chunk) ([]byte, error) {
	raw := chunkNBT(key, c)
	if raw == nil {
		return nil, nil
	}
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type encodedSection struct {
	y       int32
	palette []uint16
	data    []int64
}

func chunkNBT(key [2]int32, c *chunk) []byte {
	if c == nil {
		return nil
	}
	sections := make([]encodedSection, 0, len(c.sections))
	for y, blocks := range c.sections {
		palette, data := encodeSection(blocks)
		if palette == nil {
			continue
		}
		sections = append(sections, encodedSection{y: y, palette: palette, data: data})
	}
	if len(sections) == 0 {
		return nil
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].y < sections[j].y })

	n := &nbt{}
	n.compound("", func() {
		n.intField("DataVersion", dataVersion)
		n.intField("xPos", key[0])
		n.intField("yPos", worldMinY/16)
		n.intField("zPos", key[1])
		n.stringField("Status", "minecraft:full")
		n.longField("LastUpdate", 0)
		n.longField("InhabitedTime", 0)
		// Lighting is left to Minecraft: it relights the chunk on first load.
		n.byteField("isLightOn", 0)
		n.list("sections", tagCompound, len(sections), func() {
			for _, section := range sections {
				n.element(func() {
					n.byteField("Y", int8(section.y))
					n.compound("block_states", func() {
						n.list("palette", tagCompound, len(section.palette), func() {
							for _, block := range section.palette {
								n.element(func() { n.stringField("Name", blockName(block)) })
							}
						})
						n.longArrayField("data", section.data)
					})
					n.compound("biomes", func() {
						n.list("palette", tagString, 1, func() { n.text("minecraft:plains") })
					})
				})
			}
		})
		// An empty Heightmaps compound makes Minecraft recompute them.
		n.compound("Heightmaps", func() {})
		n.list("block_entities", tagCompound, 0, nil)
		n.list("block_ticks", tagCompound, 0, nil)
		n.list("fluid_ticks", tagCompound, 0, nil)
		n.list("PostProcessing", tagList, 0, nil)
		n.compound("structures", func() {
			n.compound("starts", func() {})
			n.compound("References", func() {})
		})
	})
	return n.bytes()
}

// encodeSection builds the section palette and the packed block indices.
// Minecraft uses at least 4 bits per entry and never splits an entry across two
// longs. A section that holds nothing but air returns a nil palette.
func encodeSection(blocks *[4096]uint16) ([]uint16, []int64) {
	indices := map[uint16]int{airBlock: 0}
	palette := []uint16{airBlock}
	for _, block := range blocks {
		if _, seen := indices[block]; !seen {
			indices[block] = len(palette)
			palette = append(palette, block)
		}
	}
	if len(palette) == 1 {
		return nil, nil
	}

	bits := 4
	for 1<<bits < len(palette) {
		bits++
	}
	perLong := 64 / bits
	data := make([]int64, (4096+perLong-1)/perLong)
	for index, block := range blocks {
		data[index/perLong] |= int64(indices[block]) << ((index % perLong) * bits)
	}
	return palette, data
}

func blockName(block uint16) string {
	if block == airBlock || int(block) > len(blocks) {
		return "minecraft:air"
	}
	return blocks[block-1].id
}

// writeLevel writes level.dat. The world is a void superflat so that only the
// exported blocks exist and Minecraft adds no terrain around them.
func (w *World) writeLevel(spawnY int32) error {
	n := &nbt{}
	n.compound("", func() {
		n.compound("Data", func() {
			n.intField("DataVersion", dataVersion)
			n.intField("version", 19133)
			n.compound("Version", func() {
				n.intField("Id", dataVersion)
				n.stringField("Name", versionName)
				n.byteField("Snapshot", 0)
			})
			n.stringField("LevelName", w.opt.Name)
			n.intField("GameType", 1)
			n.byteField("allowCommands", 1)
			n.byteField("Difficulty", 0)
			n.byteField("DifficultyLocked", 0)
			n.byteField("hardcore", 0)
			n.byteField("initialized", 1)
			n.byteField("raining", 0)
			n.byteField("thundering", 0)
			n.intField("rainTime", 100000)
			n.intField("thunderTime", 100000)
			n.intField("clearWeatherTime", 100000)
			n.intField("SpawnX", 0)
			n.intField("SpawnY", spawnY)
			n.intField("SpawnZ", 0)
			n.floatField("SpawnAngle", 0)
			n.longField("Time", 6000)
			n.longField("DayTime", 6000)
			n.longField("LastPlayed", time.Now().UnixMilli())
			n.byteField("WasModded", 1)
			n.doubleField("BorderCenterX", 0)
			n.doubleField("BorderCenterZ", 0)
			n.doubleField("BorderSize", 59999968)
			n.compound("GameRules", func() {
				n.stringField("doDaylightCycle", "false")
				n.stringField("doWeatherCycle", "false")
				n.stringField("doMobSpawning", "false")
				n.stringField("keepInventory", "true")
			})
			n.compound("DataPacks", func() {
				n.list("Enabled", tagString, 1, func() { n.text("vanilla") })
				n.list("Disabled", tagString, 0, nil)
			})
			n.compound("WorldGenSettings", func() {
				n.longField("seed", 0)
				n.byteField("generate_features", 0)
				n.byteField("bonus_chest", 0)
				n.compound("dimensions", func() {
					n.compound("minecraft:overworld", func() {
						n.stringField("type", "minecraft:overworld")
						n.compound("generator", func() {
							n.stringField("type", "minecraft:flat")
							n.compound("settings", func() {
								n.stringField("biome", "minecraft:plains")
								n.byteField("lakes", 0)
								n.byteField("features", 0)
								n.list("layers", tagCompound, 0, nil)
							})
						})
					})
				})
			})
		})
	})

	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	if _, err := writer.Write(n.bytes()); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.dir, "level.dat"), out.Bytes(), 0o644)
}
