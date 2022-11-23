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

type World struct {
    bytes [][]uint8
    w, h int
}

func NewWorld(w, h int,  c distributorChannels) *World {
    bytes := make([][]uint8, h)
    for i := range bytes {
        bytes[i] = make([]uint8, w)
    }
    for y := 0; y < w; y++ {
        for x := 0; x < h; x++ {
            number := <-c.ioInput
            bytes[y][x] = number
        }
    }
    return &World{
        bytes: bytes,
        w: w, h: h,
        }
}

func (f *World) CountAliveNeigbours(x, y int) int {
    aliveNeighbours := 0
    for i := -1; i <= 1; i++ {
        for j := -1; j <= 1; j++ {
            x:=x+i
            y:=y+j
            x += f.w
            x %= f.w
            y += f.h
            y %= f.h
            if (j != 0 || i != 0) && f.bytes[y][x] == 255 {
                aliveNeighbours++
            }
        }
    }
    return aliveNeighbours
}

func (world *World) Turn() {
    buffer := make([][]uint8, world.h)
    for i := range buffer {
        buffer[i] = make([]uint8, world.w)
    }
    for y := 0; y < world.h; y++ {
        for x := 0; x < world.w; x++ {
            alive:=world.CountAliveNeigbours(x, y)
            // any live cell with fewer than two live neighbours dies
            if (alive < 2) {
                buffer[y][x] = byte(0)
            }
            // any live cell with two or three live neighbours is unaffected
            if (alive == 2 || alive == 3) {
                buffer[y][x] = world.bytes[y][x]
            }
            // any live cell with more than three live neighbours dies
            if (alive > 3) {
                buffer[y][x] = byte(0)
            }
            // any dead cell with exactly three live neighbours becomes alive
            if (alive == 3) {
                buffer[y][x] = byte(255)
            }
        }
    }
    world.bytes = buffer
}

func distributor(p Params, c distributorChannels) {

    fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

    c.ioCommand <- ioInput

    c.ioFilename <- fileName

    world := NewWorld(p.ImageWidth, p.ImageHeight, c)

    turn := 0

    for i := 0; i < p.Turns; i++ {
        world.Turn()
        turn++
    }

    ac := []util.Cell{}
    for y := 0; y < p.ImageHeight; y++ {
        for x := 0; x < p.ImageWidth; x++ {
            if (world.bytes[y][x] == byte(255)) {
                ac = append(ac, util.Cell{X: x, Y: y})
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
