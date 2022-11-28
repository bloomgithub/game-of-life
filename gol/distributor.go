package gol

import (
    "time"
    "strconv"
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
    w, h int
}

type World struct {
    field Field
    steps []Step
}

type Result struct {
    field Field
    step Step
}

type Step struct {
    start, end int
}

// NewField returns an empty field of the specified width and height.
func NewField(w, h int) Field {
    slice := make([][]uint8, h)
    for i := range slice {
        slice[i] = make([]uint8, w)
    }
    return Field{slice: slice, w: w, h: h}
}

// Change changes the state of the specified pixel to the given value.
func (field Field) Change(x, y int, b uint8) {
    field.slice[y][x] = b
}

// Alive reports whether the specified pixel is alive.
// If the x or y coordinates are outside the field boundaries they are wrapped
// toroidally. For instance, an x value of -1 is treated as width-1.
func (field Field) Alive(x, y int) bool {
    x += field.w
    x %= field.w
    y += field.h
    y %= field.h
    return field.slice[y][x] == 255
}

// Neighbours returns the number of alive neighbours of specified pixel
func (field Field) Neighbours(x, y int) int {
    n := 0
    for i := -1; i <= 1; i++ {
        for j := -1; j <= 1; j++ {
            if (j != 0 || i != 0) && field.Alive(x+i, y+j) {
                n++
            }
        }
    }
    return n
}

// Next returns the state of the specified pixel at the next time step.
func (field Field) Next(x, y int) uint8 {
    buffer := field.slice[y][x]
    n := field.Neighbours(x, y)
    if (n < 2) || (n > 3) {
        buffer = byte(0)
    };
    if (n == 3) {
        buffer = byte(255)
    }
    return buffer
}

// NewWorld returns a new World game state with specified initial state.
func NewWorld(w, h, t int, c distributorChannels) World {
    field := NewField(w, h)
    for y := 0; y < h; y++ {
        for x := 0; x < w; x++ {
            number := <-c.ioInput
            field.slice[y][x] = number
        }
    }
    step := (h + t - 1) / t;
    steps := []Step{};
    for start := 0; start < h; start += step {
        end := start + step
        if end > h {
            end = h
        }
        steps = append(steps, Step{start: start, end: end})
    };
    return World{field: field, steps: steps}
}

// Step advances the game by one instant, recomputing and updating all cells.
func (world World) Step(steps <-chan Step, result chan<- Result) {
    for step := range steps {
        buffer := NewField(world.field.w, world.field.h)
        for y := step.start; y < step.end; y++ {
            for x := 0; x < world.field.w; x++ {
                n := world.field.Next(x, y)
                buffer.Change(x, y, n)
            }
        }
        time.Sleep(time.Millisecond)
        result <- Result{
            field: buffer,
            step: Step{start: step.start, end: step.end},
        }
    }
}

func distributor(p Params, c distributorChannels) {

    fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight);

    c.ioCommand <- ioInput;

    c.ioFilename <- fileName;

    // TODO: Create a 2D slice to store the world.

    world := NewWorld(p.ImageHeight, p.ImageWidth, p.Threads, c);

    turn := 0;

    // TODO: Execute all turns of the Game of Life.

    for i := 0; i < p.Turns; i++ {
        steps := make(chan Step)
        result := make(chan Result)

        for worker := 1; worker <= len(world.steps); worker++ {
            go world.Step(steps, result)
        };

        for _, step := range world.steps {
            steps <- step
        }
        close(steps)

        results := []Result{}
        for a := 1; a <= len(world.steps); a++ {
            buffer := <-result
            results = append(results, buffer)
        };

        for _, field := range results {
            for row := field.step.start; row < field.step.end; row++ {
                world.field.slice[row] = field.field.slice[row]
            }
        }
        turn++
    }

    // TODO: Report the final state using FinalTurnCompleteEvent.

    a := []util.Cell{}
    for y := 0; y < world.field.h; y++ {
        for x := 0; x < world.field.w; x++ {
            if world.field.Alive(x, y) {
                a = append(a, util.Cell{X: x, Y: y})
            }
        }
    };

    c.events <- FinalTurnComplete{
        CompletedTurns: turn,
        Alive: a}

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle

    c.events <- StateChange{turn, Quitting}

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)
}