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

// distributor divides the work between workers and interacts with other goroutines.

type World struct {
	field [][]uint8
    steps []Step
	height, width, threads int
}

type Step struct {
    start, end int
}

type Alive struct {
    cells []util.Cell
    count int
}

var setTurnsCh = make(chan bool)

var getTurnsCh =make(chan int)

var setCountCh = make(chan int)

var getCountCh = make(chan int)

func countingBroker(inCount int) {
    var turns int
    var count = inCount
    for {
        select {
            case <-setTurnsCh:
                turns++
            case getTurnsCh <- turns:
            case dif := <-setCountCh:
                count += dif
            case getCountCh <-count:
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

func newSteps(height, threads int) []Step {
    steps := []Step{};
    h := height
    t := threads
    start := 0
    for start < height {
        step := h/t
        steps = append(steps, Step{start: start, end: start + step})
        h = h - step
        t = t - 1
        start = start + step
    }
    return steps
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
    steps := newSteps(height, threads);

    return &World{field: field, steps: steps, height: height, width: width, threads: threads, }
}

func (world *World) updateRegion(start, end int, regionCh chan<- [][]uint8, flippedCh chan<- []util.Cell) {
    region := newField(end-start, world.width)
    flipped := []util.Cell{}
    dif := 0
    for y := start; y < end; y++ {
        for x := 0; x < world.width; x++ {
            currentCell := world.field[y][x]
            nextCell := currentCell
        	aliveNeighbours := 0;
        	for i := -1; i <= 1; i++ {
        		for j := -1; j <= 1; j++ {
                    tx := x + i
                    ty:= y + j
                    tx += world.width
                    tx %= world.width
                    ty += world.height
                    ty %= world.height
        			if (j != 0 || i != 0) && world.field[ty][tx] == 255 {
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
                flipped = append(flipped, util.Cell{x,y})
                if nextCell == 255 {
                    dif++
                } else if nextCell == 0 {
                    dif--
                }
            }
            region[y-start][x] = nextCell
        }
    }
    regionCh <- region
    flippedCh <- flipped
    setCountCh <- dif
}

func (world *World) updateWorld(turn int, c distributorChannels) {
    var newFieldData [][]uint8

    regionCh := make([]chan [][]uint8, world.threads);
    for i := range regionCh {
        regionCh[i] = make(chan [][]uint8)
    }

    flippedCh := make(chan []util.Cell)

    for w := 0; w < world.threads; w++ {
        go world.updateRegion(world.steps[w].start, world.steps[w].end, regionCh[w], flippedCh)
    }

    newFieldData = newField(0, 0);

    for i := 0; i < world.threads; i++ {
        region := <-regionCh[i];
        newFieldData = append(newFieldData, region...)
    }

    for i := 0; i < world.threads; i++ {
        flipped := <-flippedCh
        for i := range flipped {
            c.events <- CellFlipped{turn, flipped[i]}
        }
    }

    world.field = newFieldData
}

func (world *World) saveWorld(turn int, c distributorChannels) {
    outFilename := strconv.Itoa(world.width) + "x" + strconv.Itoa(world.height) + "x" + strconv.Itoa(turn)

    c.ioCommand <- ioOutput
    c.ioFilename <- outFilename

    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            c.ioOutput <- world.field[y][x]
        }
    };

    c.events <- ImageOutputComplete{
        CompletedTurns: turn,
        Filename: outFilename}
}

func (world *World) reportAlive(turn int, stopCounterCh <-chan bool, c distributorChannels) {
    ticker := time.NewTicker(2 * time.Second)

    for {
        select {
        case <-ticker.C:
            turns := <- getTurnsCh
            count := <- getCountCh
            c.events <- AliveCellsCount{
                CompletedTurns: turns,
                CellsCount: count}
            case <-stopCounterCh:
                ticker.Stop();
                return
        }
    }
}

func (world *World) liveWorld(turns int, wg *sync.WaitGroup, c distributorChannels) {
    defer wg.Done()
    paused := false
    turn := 0

    stopCounterCh := make(chan bool);
    go world.reportAlive(turn, stopCounterCh, c);

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
                            c.events <- StateChange{turn, Executing}
                            fmt.Printf("\nContinuing\n")
                        } else {
                            c.events <- StateChange{turn, Paused}
                            fmt.Printf("\nCurrent turn: %d\n", turn + 1)
                        };
                        paused = !paused
                    default:
                        paused = false
                }
            default:
                if !paused {
                    world.updateWorld(turn, c);
                    setTurnsCh <- true
                    c.events <- TurnComplete{
                        CompletedTurns: turn}
                    turn++;
                }
        }
    }
    stopCounterCh<-true
    world.saveWorld(turn, c)
}

func distributor(p Params, c distributorChannels) {
    world := loadWorld(p.ImageHeight, p.ImageWidth, p.Threads, c)

    inCount := len(world.getAlive())
    go countingBroker(inCount)

    var wg sync.WaitGroup

    wg.Add(1);

    go world.liveWorld(p.Turns, &wg, c)

    wg.Wait()

    turns := <- getTurnsCh

    c.events <- FinalTurnComplete{
        CompletedTurns: turns,
        Alive: world.getAlive()}

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle;

    c.events <- StateChange{turns, Quitting};

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)
}