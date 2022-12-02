package gol

import (
    "strconv"
	"uk.ac.bris.cs/gameoflife/util"
    "time"
    "sync"
    "fmt"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
    keyPresses <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.

type World struct {
	field [][]uint8
	height, width, threads int
}

type Step struct {
    start, end int
}

type Alive struct {
    cells []util.Cell
    count int
}

var completedTurns = 0;
var cellsCount = 0;

var paused = false;

func makeField(height, width int) [][]uint8 {
    field := make([][]uint8, height)
    for i := range field {
        field[i] = make([]uint8, width)
    }
    return field
}

func (world *World) alive() *Alive {
    cells := []util.Cell{};
    count := 0;
    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            if world.field[y][x] == 255 {
                cells = append(cells, util.Cell{X: x, Y: y})
                count++
            }
        }
    }

    return &Alive{cells: cells, count: count}
}

func (world *World) next(s, e int, c distributorChannels) [][]uint8 {
    buffer := makeField(e-s, world.width)
    for y := s; y < e; y++ {
        for x := 0; x < world.width; x++ {
            next := world.field[y][x]
        	n := 0
        	for i := -1; i <= 1; i++ {
        		for j := -1; j <= 1; j++ {
                    tx := x + i
                    ty:= y + j
                    tx += world.width
                    tx %= world.width
                    ty += world.height
                    ty %= world.height
        			if (j != 0 || i != 0) && world.field[ty][tx] == 255 {
        				n++
        			}
        		}
            }
        	if (n < 2) || (n > 3) {
                next = 0
        	}
        	if n == 3 {
                next = 255
        	}
            buffer[y-s][x] = next
            if buffer[y-s][x] != world.field[y][x] {
                c.events <- CellFlipped{
                    CompletedTurns: completedTurns,
                    Cell: util.Cell{X: x, Y: y,},
                }
            }
        }
    }
    return buffer
}

func (world *World) worker(i, s, e int, out chan<- [][]uint8, c distributorChannels){
    part := world.next(s, e, c);
    out <- part
}

func (world *World) update(c distributorChannels) {
    var newCellData [][]uint8

    out := make([]chan [][]uint8, world.threads);
    for i := range out {
        out[i] = make(chan [][]uint8)
    }

    steps := []Step{}
    h := world.height
    t := world.threads
    start := 0
    for start < world.height {
        step := h/t
        steps = append(steps, Step{start: start, end: start + step})
        h = h - step
        t = t - 1
        start = start + step
    }

    for w := 0; w < world.threads; w++ {
        go world.worker(w, steps[w].start, steps[w].end, out[w], c)
    }

    newCellData = makeField(0, 0);

    for i := 0; i < world.threads; i++ {
        part := <-out[i]
        newCellData = append(newCellData, part...)
    }

    world.field = newCellData

}

func create(height, width, threads int, c distributorChannels) *World {
    field := makeField(height, width)
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            pixel := <-c.ioInput;
            field[y][x] = pixel
            if field[y][x] == 255 {
                c.events <- CellFlipped{
                    CompletedTurns: 0,
                    Cell: util.Cell{X: x, Y: y,},
                    }
            }
        }
    };

    return &World{field: field, height: height, width: width, threads: threads, }
}

func ticker(stopTickerCh <-chan bool, c distributorChannels) {
    ticker := time.NewTicker(2 * time.Second)

    for {
        select {
            case <-ticker.C:
                c.events <- AliveCellsCount{
                    CompletedTurns: completedTurns,
                    CellsCount: cellsCount,
                }
            case <-stopTickerCh:
                ticker.Stop();
                return
        }
    }
}

func (world *World) output(c distributorChannels) {
    outFilename := strconv.Itoa(world.width) + "x" + strconv.Itoa(world.height) + "x" + strconv.Itoa(completedTurns)

    c.ioCommand <- ioOutput
    c.ioFilename <- outFilename

    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            c.ioOutput <- world.field[y][x]
        }
    };

    c.events <- ImageOutputComplete{
        CompletedTurns: completedTurns,
        Filename: outFilename,
        }
}

func (world *World) handler(turns int, wg *sync.WaitGroup, c distributorChannels) {
    defer wg.Done()
    var status = "Play"
    var turn = 0
    for turn < turns {
        select {
        case cmd := <- c.keyPresses:
            fmt.Println(cmd)
            switch cmd {
            case 's':
                world.output(c)
            case 'q':
                world.output(c)
                return;
            case 'p':
                if !paused {
                    status = "Pause"
                    c.events <- StateChange{
                        CompletedTurns: completedTurns,
                        NewState: Paused,
                        }
                    fmt.Printf("\nCurrent turn: %d\n", turn+1)
                }
                if paused {
                    status = "Play"
                    c.events <- StateChange{
                        CompletedTurns: completedTurns,
                        NewState: Executing,
                        }
                    fmt.Printf("\nContinuing\n")
                }
                paused = !paused
            default:
                status = "Play"
            }
            default:
                if status == "Play" {
                    world.update(c);
                    turn++;
                    completedTurns = turn
                    cellsCount = world.alive().count
                    c.events <- TurnComplete{
                        CompletedTurns: completedTurns,
                        }
                }
        }
    }
}

func distributor(p Params, c distributorChannels) {
    inFilename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

    c.ioCommand <- ioInput;

    c.ioFilename <- inFilename;

    world := create(p.ImageHeight, p.ImageWidth, p.Threads, c)

    stopTickerCh := make(chan bool);
    go ticker(stopTickerCh, c);

    var wg sync.WaitGroup
    wg.Add(1)
    go world.handler(p.Turns, &wg, c)
    wg.Wait()

    stopTickerCh <- true

    c.events <- FinalTurnComplete{
        CompletedTurns: completedTurns,
        Alive: world.alive().cells,
        }

    world.output(c)

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle

    c.events <- StateChange{completedTurns, Quitting}

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)

}
