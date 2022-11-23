package gol

import (
    "fmt"
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
    field [][]uint8
    w, h int
}

func NewWorld(w, h int,  c distributorChannels) *World {
    field := make([][]uint8, h)
    for i := range field {
        field[i] = make([]uint8, w)
    }
    for row := 0; row < w; row++ {
        for column := 0; column < h; column++ {
            number := <-c.ioInput
            field[row][column] = number
        }
    }
    return &World{
        field: field,
        w: w, h: h,
        }
}

func (f *World) CountAliveNeigbours(row, column int) int {
    aliveNeighbours := 0
    for i := -1; i <= 1; i++ {
        for j := -1; j <= 1; j++ {
            row:=row+i
            column:=column+j
            row += f.w
            row %= f.w
            column += f.h
            column %= f.h
            if (j != 0 || i != 0) && f.field[row][column] == 255 {
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
    for row := 0; row < world.h; row++ {
        for column := 0; column < world.w; column++ {
            alive:=world.CountAliveNeigbours(row, column)
            // any live cell with fewer than two live neighbours dies
            if (alive < 2) {
                buffer[row][column] = byte(0)
            }
            // any live cell with two or three live neighbours is unaffected
            if (alive == 2 || alive == 3) {
                buffer[row][column] = world.field[row][column]
            }
            // any live cell with more than three live neighbours dies
            if (alive > 3) {
                buffer[row][column] = byte(0)
            }
            // any dead cell with exactly three live neighbours becomes alive
            if (alive == 3) {
                buffer[row][column] = byte(255)
            }
        }
    }
    world.field = buffer
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
    for row := 0; row < p.ImageHeight; row++ {
        for column := 0; column < p.ImageWidth; column++ {
            if (world.field[row][column] == byte(255)) {
                ac = append(ac, util.Cell{X: column, Y: row})
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
