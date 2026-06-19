package api

import "math/rand"

// Thread IDs are three-word botanical names: {descriptor}-{plant}-{phenomenon}
// 31 × 41 × 36 = 45,756 unique combinations.

var threadDescriptors = []string{
	"amber", "ancient", "azure", "bronze", "copper",
	"crimson", "crystal", "dusty", "emerald", "faded",
	"gentle", "golden", "hidden", "jade", "lush",
	"misty", "mossy", "muted", "pale", "quiet",
	"russet", "sage", "shadowed", "silent", "silver",
	"soft", "still", "sunlit", "tangled", "velvet",
	"verdant",
}

var threadPlants = []string{
	"alder", "aloe", "ash", "bark", "birch",
	"bloom", "blossom", "bramble", "briar", "cedar",
	"clover", "cypress", "dahlia", "elm", "fern",
	"fig", "flora", "frond", "heath", "holly",
	"iris", "ivy", "juniper", "laurel", "lotus",
	"maple", "mint", "moss", "myrtle", "oak",
	"orchid", "pine", "reed", "rose", "sage",
	"thyme", "vine", "violet", "willow", "wisteria",
	"yarrow",
}

var threadPhenomena = []string{
	"bloom", "bower", "brook", "cascade", "clearing",
	"dawn", "dew", "drift", "dusk", "fall",
	"fog", "frost", "glade", "glow", "grove",
	"haze", "light", "mist", "morning", "rain",
	"reach", "rest", "rise", "shade", "shimmer",
	"spring", "still", "storm", "stream", "sway",
	"tide", "trace", "vale", "warmth", "wave",
	"wind",
}

// generateThreadID returns a random botanical three-word thread ID,
// e.g. "silver-fern-cascade".
func generateThreadID() string {
	d := threadDescriptors[rand.Intn(len(threadDescriptors))]
	p := threadPlants[rand.Intn(len(threadPlants))]
	ph := threadPhenomena[rand.Intn(len(threadPhenomena))]
	return d + "-" + p + "-" + ph
}
