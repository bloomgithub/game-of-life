package gol

import (
    "strconv"
    "sync"
    "fmt"
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
    keyPresses <-chan rune
}

var setCompletedTurnsCh = make(chan bool)

var getCompletedTurnsCh = make(chan int)

var setCellsCountCh = make(chan int)

var getCellsCountCh = make(chan int)

type World struct {
	field [][]uint8
	height, width, threads int
}

type Region struct {
    field [][]uint8
    start, end, height, width int
}

func countingBroker(stopBrokerCh <-chan bool) {
    var completedTurns int
    var cellsCount = 0
    for {
        select {
            case <- setCompletedTurnsCh:
                completedTurns++
            case getCompletedTurnsCh <- completedTurns:
            case newCount := <- setCellsCountCh:
                cellsCount = newCount
            case getCellsCountCh <- cellsCount:
            case <- stopBrokerCh:
                return
        }
    }
}

func reportAlive(stopReporterCh <-chan bool, c distributorChannels) {
    ticker := time.NewTicker(2 * time.Second)

    for {
        select {
            case <-ticker.C:
                completedTurns := <- getCompletedTurnsCh
                cellsCount := <- getCellsCountCh
                c.events <- AliveCellsCount{
                    CompletedTurns: completedTurns,
                    CellsCount: cellsCount,
                }
            case <- stopReporterCh:
                ticker.Stop();
                return
        }
    }
}

func newField(height, width int) [][]uint8 {
    field := make([][]uint8, height)
    for i := range field {
        field[i] = make([]uint8, width)
    };
    return field
}

func (world *World) saveWorld(turn int, c distributorChannels) {
    filename := strconv.Itoa(world.width) + "x" + strconv.Itoa(world.height) + "x" + strconv.Itoa(turn)

    c.ioCommand <- ioOutput
    c.ioFilename <- filename

    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            c.ioOutput <- world.field[y][x]
        }
    };

    c.events <- ImageOutputComplete{
        CompletedTurns: turn,
        Filename: filename,
    }
}

func (world *World) getAlive() []util.Cell {
    alive := []util.Cell{}
    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            if world.field[y][x] == 255 {
                alive= append(alive, util.Cell{X: x, Y: y})
            }
        }
    }
    return alive
}

func (region *Region) updateRegion(regionCh chan<- [][]uint8, flippedCh chan<- []util.Cell) {
    field := newField(region.height, region.width)
    haloOffset := 1
    flipped := []util.Cell{}
    for y := haloOffset; y < region.height + haloOffset; y++ {
        for x := 0; x < region.width; x++ {
            currentCell := region.field[y][x]
            nextCell := currentCell
        	aliveNeighbours := 0;
            for i := -1; i <= 1; i++ {
                for j := -1; j <= 1; j++ {
                    wx := x + i
                    wy := y + j
                    wx += region.width
                    wx %= region.width
                    if (j != 0 || i != 0) && region.field[wy][wx] == 255 {
                        aliveNeighbours++
                    }
                }
            }
            if (aliveNeighbours < 2) || (aliveNeighbours > 3) {
                nextCell = 0
        	}
            if aliveNeighbours == 3 {
                nextCell = 255
            }
            if nextCell != currentCell {
                flipped = append(flipped, util.Cell{
                    X: x,
                    Y: y - haloOffset + region.start,
                })
            }
            field[y-haloOffset][x] = nextCell
        }
    }
    regionCh <- field
    flippedCh <- flipped
}

func (world *World) makeHalo(w int) Region {
     field := newField(0, 0)

     regionHeight := world.height/world.threads
     start := w * regionHeight
     end := (w + 1) * regionHeight
     if w == world.threads - 1 {
         end = world.height
     }
     regionHeight = end - start

     downRowPtr := end % world.height

     upRowPtr := start - 1
     upRowPtr = upRowPtr + world.height
     upRowPtr = upRowPtr % world.height

     field = append(field, world.field[upRowPtr])
     for row := start; row < end; row++ {
        field = append(field, world.field[row])
     }
     field = append(field, world.field[downRowPtr])

     return Region{
         field: field,
         start: start,
         end: end,
         height: regionHeight,
         width: world.width,
     }
}

func (world *World) updateWorld(turn int, c distributorChannels) {
    var newFieldData [][]uint8

    regionCh := make([]chan [][]uint8, world.threads);
    for i := range regionCh {
        regionCh[i] = make(chan [][]uint8)
    }

    flippedCh := make(chan []util.Cell)

   for w := 0; w < world.threads; w++ {
       region := world.makeHalo(w)
       go region.updateRegion(regionCh[w], flippedCh)
    }

    newFieldData = newField(0, 0);
    for i := 0; i < world.threads; i++ {
        region := <-regionCh[i];
        newFieldData = append(newFieldData, region...)
        flipped := <-flippedCh
        for f := range flipped {
            c.events <- CellFlipped{
                CompletedTurns: turn,
                Cell: flipped[f],
            }
        }
    }

    world.field = newFieldData
}

func loadWorld(height, width, threads int, c distributorChannels) *World {
    inFilename := strconv.Itoa(width) + "x" + strconv.Itoa(height)

    c.ioCommand <- ioInput;

    c.ioFilename <- inFilename;

    field := newField(height, width)
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            p := <-c.ioInput;
            field[y][x] = p
            if p == 255 {c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}}
        }
    };

    return &World{field: field, height: height, width: width, threads: threads, }
}

func (world *World) liveWorld(turns int, wg *sync.WaitGroup, c distributorChannels) {
    defer wg.Done()
    paused := false
    turn := 0
    for turn < turns {
        select {
            case cmd := <- c.keyPresses:
                switch cmd {
                case 's':
                    world.saveWorld(turn, c)
                case 'q':
                    world.saveWorld(turn, c)
                    return;
                case 'p':
                    if paused {
                        c.events <- StateChange{
                            CompletedTurns: turn,
                            NewState: Executing,
                        }
                        fmt.Printf("\nContinuing\n")
                    } else {
                        c.events <- StateChange{
                            CompletedTurns: turn,
                            NewState: Paused,
                        }
                        fmt.Printf("\nCurrent turn: %d\n", turn + 1)
                    }
                    paused = !paused
                default:
                    paused = false
                }
                default:
                    if !paused {
                        world.updateWorld(turn, c);
                        setCellsCountCh <- len(world.getAlive())
                        setCompletedTurnsCh <- true
                        c.events <- TurnComplete{
                            CompletedTurns: turn,
                        }
                        turn++;
                    }
        }
    }
    world.saveWorld(turn, c)
}

func distributor(p Params, c distributorChannels) {
    world := loadWorld(p.ImageHeight, p.ImageWidth, p.Threads, c)

    var stopReporterCh = make(chan bool)
    go reportAlive(stopReporterCh, c)

    var stopBrokerCh = make(chan bool)
    go countingBroker(stopBrokerCh)

    var wg sync.WaitGroup

    wg.Add(1);

    go world.liveWorld(p.Turns, &wg, c)

    wg.Wait()

    stopReporterCh <- true

    completedTurns := <- getCompletedTurnsCh

    stopBrokerCh <- true

    c.events <- FinalTurnComplete{
        CompletedTurns: completedTurns,
        Alive: world.getAlive(),
    }

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle;

    c.events <- StateChange{
        CompletedTurns: completedTurns,
        NewState: Quitting,
    }

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)
}