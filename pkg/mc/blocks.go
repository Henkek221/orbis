package mc

// blockColor is a full solid cube with a roughly flat average colour.
type blockColor struct {
	id      string
	r, g, b uint8
}

// Photogrammetry textures are mostly muted greys, roof reds, asphalt and
// vegetation, so the palette leans on concrete, terracotta and stone tones.
// Only full opaque cubes are listed: no stairs, slabs, glass or leaves.
var blocks = []blockColor{
	{"minecraft:white_concrete", 207, 213, 214},
	{"minecraft:light_gray_concrete", 125, 125, 115},
	{"minecraft:gray_concrete", 54, 57, 61},
	{"minecraft:black_concrete", 8, 10, 15},
	{"minecraft:brown_concrete", 96, 59, 31},
	{"minecraft:red_concrete", 142, 32, 32},
	{"minecraft:orange_concrete", 224, 97, 1},
	{"minecraft:yellow_concrete", 240, 175, 21},
	{"minecraft:lime_concrete", 94, 168, 24},
	{"minecraft:green_concrete", 73, 91, 36},
	{"minecraft:cyan_concrete", 21, 119, 136},
	{"minecraft:light_blue_concrete", 35, 137, 198},
	{"minecraft:blue_concrete", 44, 46, 143},
	{"minecraft:purple_concrete", 100, 31, 156},
	{"minecraft:magenta_concrete", 169, 48, 159},
	{"minecraft:pink_concrete", 213, 101, 142},
	{"minecraft:white_terracotta", 209, 178, 161},
	{"minecraft:light_gray_terracotta", 135, 106, 97},
	{"minecraft:gray_terracotta", 58, 42, 36},
	{"minecraft:brown_terracotta", 77, 51, 36},
	{"minecraft:red_terracotta", 143, 61, 47},
	{"minecraft:orange_terracotta", 161, 83, 37},
	{"minecraft:yellow_terracotta", 186, 133, 35},
	{"minecraft:green_terracotta", 76, 83, 42},
	{"minecraft:terracotta", 152, 94, 67},
	{"minecraft:smooth_stone", 158, 158, 158},
	{"minecraft:stone", 125, 125, 125},
	{"minecraft:andesite", 136, 136, 136},
	{"minecraft:diorite", 189, 189, 189},
	{"minecraft:calcite", 223, 224, 220},
	{"minecraft:deepslate", 77, 77, 80},
	{"minecraft:blackstone", 42, 35, 40},
	{"minecraft:basalt", 80, 78, 84},
	{"minecraft:gravel", 131, 127, 126},
	{"minecraft:clay", 160, 166, 179},
	{"minecraft:sand", 219, 207, 163},
	{"minecraft:sandstone", 216, 203, 155},
	{"minecraft:packed_mud", 150, 103, 76},
	{"minecraft:mud_bricks", 137, 110, 93},
	{"minecraft:oak_planks", 162, 130, 78},
	{"minecraft:spruce_planks", 114, 84, 48},
	{"minecraft:moss_block", 89, 109, 45},
	{"minecraft:grass_block", 91, 123, 54},
	{"minecraft:mossy_cobblestone", 110, 118, 90},
	{"minecraft:iron_block", 220, 220, 220},
	{"minecraft:dirt", 134, 96, 67},
}

// Block values are palette indices shifted by one so that zero means air.
const airBlock uint16 = 0

var (
	stoneBlock     = blockByID("minecraft:stone")
	deepslateBlock = blockByID("minecraft:deepslate")
	dirtBlock      = blockByID("minecraft:dirt")
	grassBlock     = blockByID("minecraft:grass_block")
	unknownBlock   = blockByID("minecraft:light_gray_concrete")
)

func blockByID(id string) uint16 {
	for index, block := range blocks {
		if block.id == id {
			return uint16(index) + 1
		}
	}
	panic("unknown block id " + id)
}

// isGreen reports a vegetation-like colour, used to put grass on top faces.
func isGreen(r, g, b uint8) bool {
	return int(g) > int(r)+10 && int(g) > int(b)+14
}
