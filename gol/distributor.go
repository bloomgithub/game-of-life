package gol

import (
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
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

// Field represents a 2D field of pixels
type Field struct {
	slice [][]uint8
	w, h  int
}

type World struct {
	field Field
	steps []Step
	alive int
}

type Result struct {
	field Field
	step  Step
	alive int
}

type Step struct {
	start, end int
}

type AliveCells struct {
	cells []util.Cell
	count int
}

var globalAlive = 0

var globalTurn = 0

// NewField returns an empty field of the specified width and height.
func NewField(w, h int) Field {
	slice := make([][]uint8, h)
	for i := range slice {
		slice[i] = make([]uint8, w)
	}
	return Field{slice: slice, w: w, h: h}
}

// SetCell changes the state of the specified pixel to the given value.
func (field Field) SetCell(x, y int, b uint8) {
	field.slice[y][x] = b
}

// AliveCell reports whether the specified pixel is alive.
// If the x or y coordinates are outside the field boundaries they are wrapped
// toroidally. For instance, an x value of -1 is treated as width-1.
func (field Field) AliveCell(x, y int) bool {
	x += field.w
	x %= field.w
	y += field.h
	y %= field.h
	return field.slice[y][x] == 255
}

// AliveCells returns the state of alive cells of specified World game state
func (world World) AliveCells() AliveCells {
	cells := []util.Cell{}
	count := 0
	for y := 0; y < world.field.h; y++ {
		for x := 0; x < world.field.w; x++ {
			if world.field.AliveCell(x, y) {
				cells = append(cells, util.Cell{X: x, Y: y})
				count++
			}
		}
	}
	return AliveCells{cells: cells, count: count}
}

// Neighbours returns the number of alive neighbours of specified pixel
func (field Field) Neighbours(x, y int) int {
	n := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if (j != 0 || i != 0) && field.AliveCell(x+i, y+j) {
				n++
			}
		}
	}
	return n
}

// Next returns the state of the specified pixel at the next time step.
func (field Field) Next(x, y int) uint8 {
	pixel := field.slice[y][x]
	n := field.Neighbours(x, y)
	if (n < 2) || (n > 3) {
		pixel = byte(0)
	}
	if n == 3 {
		pixel = byte(255)
	}
	return pixel
}

// NewSteps returns the slice pointers Game world state is broken into for specified thread count
func (field Field) NewSteps(threads int) []Step {
	steps := []Step{}
	step := (field.h + threads - 1) / threads
	for start := 0; start < field.h; start += step {
		end := start + step
		if end > field.h {
			end = field.h
		}
		steps = append(steps, Step{start: start, end: end})
	}
	return steps
}

// NewWorld returns a new World game state with specified initial state.
func NewWorld(w, h, t int, c distributorChannels) World {
	field := NewField(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			number := <-c.ioInput
			if number == byte(255) {
				c.events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: x, Y: y},
				}
			}
			field.SetCell(x, y, number)
		}
	}
	return World{field: field, steps: field.NewSteps(t)}
}

// Step advances the game by one instant, recomputing and updating all cells within specified range of Game world state
func (world World) Step(turn int, steps <-chan Step, result chan<- Result, c distributorChannels) {
	for step := range steps {
		buffer := NewField(world.field.w, world.field.h)
		for y := step.start; y < step.end; y++ {
			for x := 0; x < world.field.w; x++ {
				n := world.field.Next(x, y)
				if n != buffer.slice[y][x] {
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           util.Cell{X: x, Y: y},
					}
				}
				buffer.SetCell(x, y, n)
			}
		}
		result <- Result{
			field: buffer,
			step:  Step{start: step.start, end: step.end},
		}
	}
}

// Turn advances the game by one instant, recomputing and updating all cells in parralel for specified Thread count
func (world World) Turn(turn int, c distributorChannels) {
	steps := make(chan Step)
	result := make(chan Result)

	for worker := 1; worker <= len(world.steps); worker++ {
		go world.Step(turn, steps, result, c)
	}

	for _, step := range world.steps {
		steps <- step
	}
	close(steps)

	results := []Result{}
	for a := 1; a <= len(world.steps); a++ {
		buffer := <-result
		results = append(results, buffer)
	}

	for _, result := range results {
		for row := result.step.start; row < result.step.end; row++ {
			world.field.slice[row] = result.field.slice[row]
		}
	}

	c.events <- TurnComplete{
		CompletedTurns: turn,
	}

	globalAlive = world.AliveCells().count

	globalTurn++
}

func Ticker(stopTicker <-chan bool, c distributorChannels) {
	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{
				CompletedTurns: globalTurn,
				CellsCount:     globalAlive,
			}
		case <-stopTicker:
			ticker.Stop()
			return
		}
	}
}
func sendOutput(c distributorChannels, world World, p Params) {
	outFilename := strconv.Itoa(world.field.w) + "x" + strconv.Itoa(world.field.h)

	c.ioCommand <- ioOutput
	c.ioFilename <- outFilename
	//c.events <- ImageOutputComplete{p.Turns, outFilename} DO I NEED?

	for y := 0; y < world.field.h; y++ {
		for x := 0; x < world.field.w; x++ {
			c.ioOutput <- world.field.slice[y][x]
		}
	}
}

func keyHandling(k <-chan rune, c distributorChannels, world World, p Params) {
	for {
		key := <-k
		switch key {
		case 's':
			sendOutput(c, world, p)
		case 'q':
			sendOutput(c, world, p)
			c.events <- StateChange{p.Turns, Quitting}
		case 'p':
			c.ioCommand <- ioCheckIdle
			paused := <-c.ioIdle
			if paused == false {
				c.events <- StateChange{p.Turns, Paused}
			}
			if paused == true {
				c.events <- StateChange{p.Turns, Executing}
			}
		}
	}
}

func distributor(p Params, c distributorChannels, k <-chan rune) {

	stopTicker := make(chan bool)
	go Ticker(stopTicker, c)

	InFileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

	c.ioCommand <- ioInput

	c.ioFilename <- InFileName

	world := NewWorld(p.ImageHeight, p.ImageWidth, p.Threads, c)
	go keyHandling(k, c, world, p)

	turn := 0

	for i := 0; i < p.Turns; i++ {
		world.Turn(turn, c)
		turn++
	}

	c.events <- FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          world.AliveCells().cells,
	}

	stopTicker <- true

	outFilename := strconv.Itoa(world.field.w) + "x" + strconv.Itoa(world.field.h) + "x" + strconv.Itoa(turn)

	c.ioCommand <- ioOutput

	c.ioFilename <- outFilename

	for y := 0; y < world.field.h; y++ {
		for x := 0; x < world.field.w; x++ {
			c.ioOutput <- world.field.slice[y][x]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
