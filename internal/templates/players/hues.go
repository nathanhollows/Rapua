package templates

// Oklch degrees, fixed so each is contrast-checked once and neighbours stay
// distinguishable: two sections a few degrees apart cannot be told apart on a
// phone outdoors.
var questHues = []int{15, 60, 105, 150, 195, 240, 285, 330}

// HueFor goes by position alone so a section keeps its colour all run: counting
// only visible sections would recolour the rest each time one unlocked.
func HueFor(position int) int {
	n := len(questHues)
	return questHues[((position%n)+n)%n]
}
