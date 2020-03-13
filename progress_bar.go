package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/morikuni/aec"
)

type progressBar struct {
	bar   aec.ANSI
	max   uint
	right aec.ANSI
	width uint
}

/*
Remove erases the progress bar from the line where it was displayed.
 */
func (pb progressBar) Remove() aec.ANSI {
	return aec.EraseLine(aec.EraseModes.All)
}

/*
Render return the progress bar at the current progression status.
 */
func (pb *progressBar) Render(current float64) string {
	ratio := int(math.Round(float64((pb.width-2)*pb.max) / float64(pb.max)))
	res := fmt.Sprintf("[%s%s %d/%d%s\n",
		pb.bar.Apply(strings.Repeat("=", ratio)),
		pb.right.Apply("]"),
		int(math.Round(current)),
		pb.max,
		aec.Up(1),
	)
	return res
}

/*
makeProgressBar creates a progress bar.

  - width is the maximum number of progress steps;
  - max is the value for which the bar displays as full.
 */
func makeProgressBar(width uint, max uint32) progressBar {
	pb := progressBar{
		bar:   aec.Color8BitF(aec.NewRGB8Bit(255, 96, 51)),
		max:   uint(max),
		right: aec.Column(width),
		width: width,
	}
	return pb
}

