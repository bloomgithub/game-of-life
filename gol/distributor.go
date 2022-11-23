package gol

import (
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

type Matrix struct {
    bytes [][]uint8
    w, h int
}

type World struct {
    actual, buffer *Matrix
    w, h int
}

func NewMatrix(w, h int) *Matrix {
    bytes := make([][]uint8, h)
    for i := range bytes {
        bytes[i] = make([]uint8, w)
    }

    return &Matrix{bytes: bytes, w: w, h: h}
}

func (f *Matrix) Set(x, y int, b uint8) {
    f.bytes[y][x] = b
}

func NewWorld(w, h int,  c distributorChannels) *World {
    actual := NewMatrix(w, h)
    for y := 0; y < w; y++ {
        for x := 0; x < h; x++ {
            number := <-c.ioInput
            actual.Set(y, x, number)
        }
    }
    return &World{
        actual: actual, buffer: NewMatrix(w, h),
        w: w, h: h,
        }
}

func (f *Matrix) Alive(x, y int) bool {
    x += f.w
    x %= f.w
    y += f.h
    y %= f.h
    return f.bytes[y][x] == 255
}

func (f *Matrix) CountAliveNeigbours(x, y int) int {
    alive := 0
    for i := -1; i <= 1; i++ {
        for j := -1; j <= 1; j++ {
            if (j != 0 || i != 0) && f.Alive(x+i, y+j) {
                alive++
            }
        }
    }
    return alive
}

func (world *World) Step() {
    for y := 0; y < world.h; y++ {
        for x := 0; x < world.w; x++ {
            alive:=world.actual.CountAliveNeigbours(x, y)
            // any live cell with fewer than two live neighbours dies
            if (alive < 2) {
                world.buffer.Set(x, y, byte(0))
            }
            // any live cell with two or three live neighbours is unaffected
            if (alive == 2 || alive == 3) {
                world.buffer.Set(x, y, world.actual.bytes[y][x])
            }
            // any live cell with more than three live neighbours dies
            if (alive > 3) {
                world.buffer.Set(x, y, byte(0))
            }
            // any dead cell with exactly three live neighbours becomes alive
            if (alive == 3) {
                world.buffer.Set(x, y, byte(255))
            }
        }
    }
    world.actual, world.buffer = world.buffer, world.actual
}

func distributor(p Params, c distributorChannels) {

    fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

    c.ioCommand <- ioInput

    c.ioFilename <- fileName

    w:=p.ImageWidth
    h:=p.ImageHeight
    world := NewWorld(w, h, c)

    turn := 0

    for i := 0; i < p.Turns; i++ {
        world.Step()
        turn++
    }

    ac := []util.Cell{}
    for y := 0; y < p.ImageHeight; y++ {
        for x := 0; x < p.ImageWidth; x++ {
            if (world.actual.bytes[y][x] == byte(255)) {
                ac = append(ac, util.Cell{X: y, Y: x})
            }
        }
    }

    f := FinalTurnComplete{
        CompletedTurns: turn,
        Alive: ac,
    }

    c.events <- f

    // TODO: Report the final state using FinalTurnCompleteEvent.

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle

    c.events <- StateChange{turn, Quitting}

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)
}