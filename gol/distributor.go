package gol

import "fmt"

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	// workout filename from the parameters coming in
	fileName := string(p.ImageWidth) + "x" + string(p.ImageHeight) + "pgm"

	//send filename down appropriate channel
	c.ioFilename <- fileName

	//create 2D slice to store the image
	var world [][]uint8
	row1 := make([]uint8, p.ImageWidth)
	row2 := make([]uint8, p.ImageWidth)
	world = append(world, row1)
	world = append(world, row2)

	fmt.Print(world)

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	// TODO: Report the final state using FinalTurnCompleteEvent.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
