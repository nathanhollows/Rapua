package templates

// questHues is the palette a section's colour comes from, in oklch hue degrees.
//
// Fixed so each hue is contrast-checked once (hues_test.go) and neighbours stay
// distinguishable: a free choice lets two sections sit a few degrees apart,
// which nobody can tell apart on a phone outdoors.
var questHues = []int{15, 60, 105, 150, 195, 240, 285, 330}

// HueFor is the hue of the section at position in the authored tree, counting
// from zero.
//
// Position alone, so a section keeps its colour for the whole run: counting only
// visible sections would recolour the rest each time one unlocked.
func HueFor(position int) int {
	n := len(questHues)
	return questHues[((position%n)+n)%n]
}
