package gol

import (
    "strconv"
	"uk.ac.bris.cs/gameoflife/util"
    "sync"
    "fmt"
    "time"
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
	field [][]bool
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

var completedTurns = 0;

func newField(height, width int) [][]bool {
    field := make([][]bool, height)
    for i := range field {
        field[i] = make([]bool, width)
    };
    return field
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
            b := p == 255
            field[y][x] = b
            if b {c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}}
        }
    };
    steps := newSteps(height, threads);

    return &World{field: field, steps: steps, height: height, width: width, threads: threads, }
}

func (world *World) updateRegion(start, end int, regionCh chan<- [][]bool, flippedCh chan<- []util.Cell) {
    region := newField(end-start, world.width)
    flipped := []util.Cell{}
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
        			if (j != 0 || i != 0) && world.field[ty][tx] {
                        aliveNeighbours++
        			}
        		}
            }
            if (aliveNeighbours < 2) || (aliveNeighbours > 3) {
                nextCell = false
        	}
            if aliveNeighbours == 3 {
                nextCell = true
            }
            if nextCell != currentCell {
                flipped = append(flipped, util.Cell{x,y})
            }
            region[y-start][x] = nextCell
        }
    }
    regionCh <- region
    flippedCh <- flipped
}

func (world *World) updateWorld(turn int, c distributorChannels) {
    var newFieldData [][]bool

    regionCh := make([]chan [][]bool, world.threads);
    for i := range regionCh {
        regionCh[i] = make(chan [][]bool)
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
            c.events <- CellFlipped{completedTurns, flipped[i]}
        }
    }

    world.field = newFieldData

    c.events <- TurnComplete{turn}

    completedTurns = turn + 1
}

func (world *World) getAlive() *Alive {
    cells := []util.Cell{};
    count := 0;
    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            if world.field[y][x] {
                cells = append(cells, util.Cell{X: x, Y: y})
                count++
            }
        }
    }

    return &Alive{cells: cells, count: count}
}


func (world *World) aliveCounter(turn int, stopCounterCh <-chan bool, c distributorChannels) {
    ticker := time.NewTicker(2 * time.Second)

    for {
        select {
        case <-ticker.C:
            c.events <- AliveCellsCount{completedTurns, world.getAlive().count}
            case <-stopCounterCh:
                ticker.Stop();
                return
        }
    }
}

func (world *World) saveWorld(turn int, c distributorChannels) {
    outFilename := strconv.Itoa(world.width) + "x" + strconv.Itoa(world.height) + "x" + strconv.Itoa(turn)

    c.ioCommand <- ioOutput
    c.ioFilename <- outFilename

    for y := 0; y < world.height; y++ {
        for x := 0; x < world.width; x++ {
            out := byte(0)
            if world.field[y][x]{
                out = byte(255)
            }
            c.ioOutput <- out
        }
    };

    c.events <- ImageOutputComplete{turn, outFilename}
}

func (world *World) liveWorld(turns int, wg *sync.WaitGroup, c distributorChannels) {
    defer wg.Done()
    paused := false
    turn := 0

    stopCounterCh := make(chan bool);
    go world.aliveCounter(turn, stopCounterCh, c);

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
                            c.events <- StateChange{completedTurns, Executing}
                            fmt.Printf("\nContinuing\n")
                        } else {
                            c.events <- StateChange{completedTurns, Paused}
                            fmt.Printf("\nCurrent turn: %d\n", turn+1)
                        };
                        paused = !paused
                    default:
                        paused = false
                }
            default:
                if !paused {
                    world.updateWorld(turn, c);
                    turn++;
                }
        }
    }
    stopCounterCh<-true
    world.saveWorld(turn, c)
}

func distributor(p Params, c distributorChannels) {
    world := loadWorld(p.ImageHeight, p.ImageWidth, p.Threads, c)


    var wg sync.WaitGroup

    wg.Add(1);

    go world.liveWorld(p.Turns, &wg, c)

    wg.Wait()

    c.events <- FinalTurnComplete{completedTurns, world.getAlive().cells}

    // Make sure that the Io has finished any output before exiting.
    c.ioCommand <- ioCheckIdle
    <-c.ioIdle;

    c.events <- StateChange{completedTurns, Quitting};

    // Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
    close(c.events)
}