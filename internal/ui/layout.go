package ui

// layout holds exact panel dimensions for the current terminal size.
type layout struct {
	termW int
	termH int

	statusH  int
	contentH int
	brokerH  int
	inputH   int

	leftW    int
	agent1W  int
	agent2W  int
}

// computeLayout calculates panel sizes to fill the terminal exactly.
func computeLayout(termW, termH int) layout {
	statusH := 1
	brokerH := 4
	inputH := 2
	contentH := termH - statusH - brokerH - inputH
	if contentH < 4 {
		contentH = 4
	}

	leftW := int(float64(termW) * 0.15)
	if leftW < 12 {
		leftW = 12
	}
	// keep a sane max so agents always have room
	if leftW > 30 {
		leftW = 30
	}

	rightW := termW - leftW
	agent1W := rightW / 2
	agent2W := rightW - agent1W

	return layout{
		termW:    termW,
		termH:    termH,
		statusH:  statusH,
		contentH: contentH,
		brokerH:  brokerH,
		inputH:   inputH,
		leftW:    leftW,
		agent1W:  agent1W,
		agent2W:  agent2W,
	}
}
