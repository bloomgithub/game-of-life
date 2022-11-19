package gol

import (
	"fmt"
	"strconv"
)

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

	// filename from the parameters
	fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

	//send command
	c.ioCommand <- ioInput

	//send filename
	c.ioFilename <- fileName

	//create 2D slice to store the world
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	//get image byte by byte and store in 2D world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			number := <-c.ioInput
			world[y][x] = number
		}
	}

	turn := 0

	fmt.Println(p.Turns)

	// TODO: Execute all turns of the Game of Life.

	func calculateNextState(p Params, world [][]byte) [][]byte {
		var neighboursToCheck []cell
		var cellsToTurnOff []cell
		var cellsToTurnOn []cell
		//Variations to check:w
		neighboursToCheck = append(neighboursToCheck, cell{1, 0}) //right
		neighboursToCheck = append(neighboursToCheck, cell{-1, 0})
		neighboursToCheck = append(neighboursToCheck, cell{0, -1})
		neighboursToCheck = append(neighboursToCheck, cell{0, 1})
		neighboursToCheck = append(neighboursToCheck, cell{1, -1})
		neighboursToCheck = append(neighboursToCheck, cell{-1, -1})
		neighboursToCheck = append(neighboursToCheck, cell{1, 1})
		neighboursToCheck = append(neighboursToCheck, cell{-1, 1})

		for widthVar := 0; widthVar < p.imageWidth; widthVar++ {
		for heightVar := 0; heightVar < p.imageHeight; heightVar++ {
		var liveNeighbours int = 0
		currentCell := cell{widthVar, heightVar}

		for _, element := range neighboursToCheck {
		if world[golModulus(currentCell.x+element.x, p.imageWidth)][golModulus(currentCell.y+element.y, p.imageHeight)] == 255 {
		liveNeighbours++
	}
	}
		if world[currentCell.x][currentCell.y] == 255 && (liveNeighbours < 2 || liveNeighbours > 3) {
		cellsToTurnOff = append(cellsToTurnOff, cell{currentCell.x, currentCell.y})
	}
		if world[currentCell.x][currentCell.y] == 0 && liveNeighbours == 3 {
		cellsToTurnOn = append(cellsToTurnOn, cell{currentCell.x, currentCell.y})
	}
	}
	}
		//any live cell with fewer than two live neighbours dies
		//any live cell with two or three live neighbours is unaffected
		//any live cell with more than three live neighbours dies
		//any dead cell with exactly three live neighbours becomes alive

		//HERE we apply the changes requested and return the updated world.
		for _, element := range cellsToTurnOn {
		world[element.x][element.y] = 255
	}
		for _, element := range cellsToTurnOff {
		world[element.x][element.y] = 0
	}
		return world
	}

	func calculateAliveCells(p golParams, world [][]byte) []cell {
		var cells []cell
		for i := 0; i < p.imageWidth; i++ {
		for j := 0; j < p.imageHeight; j++ {
		if world[i][j] == 255 {
		cells = append(cells, cell{
		x: j,
		y: i,
	})
	}
	}
	}
		return cells
	}

	func golModulus(inputCoord int, bound int) int {
		newCoord := inputCoord
		if inputCoord < 0 {
		newCoord = bound - 1
	}
		if inputCoord >= bound {
		newCoord = 0
	}
		return newCoord
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
