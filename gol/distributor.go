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

// Field represents a two-dimensional field of cells.
type Field struct {
    s    [][]uint8
    w, h int
}

type Life struct {
    a, b *Field
    w, h int
}

func NewField(w, h int) *Field {
    s := make([][]uint8, h)
    for i := range s {
        s[i] = make([]uint8, w)
    }

    return &Field{s: s, w: w, h: h}
}

func (f *Field) Set(x, y int, b uint8) {
    f.s[y][x] = b
}

func NewLife(w, h int,  c distributorChannels) *Life {
    a := NewField(w, h)
    for i := 0; i < w; i++ {
        for j := 0; j < h; j++ {
            number := <-c.ioInput
            a.Set(i, j, number)
        }
    }
    return &Life{
        a: a, b: NewField(w, h),
        w: w, h: h,
        }
}

func (f *Field) Alive(x, y int) bool {
    x += f.w
    x %= f.w
    y += f.h
    y %= f.h
    return f.s[y][x] == 255
}

func (f *Field) Next(x, y int) int {
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

func (l *Life) Step() {
    for y := 0; y < l.h; y++ {
        for x := 0; x < l.w; x++ {
            alive:=l.a.Next(x, y)
            // any live cell with fewer than two live neighbours dies
            if (alive < 2) {
                l.b.Set(x, y, byte(0))
            }
            // any live cell with two or three live neighbours is unaffected
            if (alive == 2 || alive == 3) {
                l.b.Set(x, y, l.a.s[y][x])
            }
            // any live cell with more than three live neighbours dies
            if (alive > 3) {
                l.b.Set(x, y, byte(0))
            }
            // any dead cell with exactly three live neighbours becomes alive
            if (alive == 3) {
                l.b.Set(x, y, byte(255))
            }
        }
    }
    l.a, l.b = l.b, l.a
}

func distributor(p Params, c distributorChannels) {

    fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

    c.ioCommand <- ioInput

    c.ioFilename <- fileName

    w:=p.ImageWidth
    h:=p.ImageHeight
    l := NewLife(w, h, c)

    turn := 0

    for i := 0; i < p.Turns; i++ {
        l.Step()
        turn++
    }

    ac := []util.Cell{}
    for y := 0; y < p.ImageHeight; y++ {
        for x := 0; x < p.ImageWidth; x++ {
            if (l.a.s[y][x] == byte(255)) {
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
