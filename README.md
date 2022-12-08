Stage 1 - Parallel Implementation
In this stage, you are required to write code to evolve Game of Life using multiple worker goroutines on a single machine. Below are some suggested steps to help you get started. You are not required to follow them. Your implementation will be marked against the success criteria outlined below.

Step 1
Implement the Game of Life logic as it was described in the task introduction. We suggest starting with a single-threaded implementation that will serve as a starting point in subsequent steps. Your Game of Life should evolve for the number of turns specified in gol.Params.Turns. Your Game of Life should evolve the correct image specified by gol.Params.ImageWidth and gol.Params.ImageHeight.

The skeleton code starts three goroutines. The diagram below shows how they should interact with each other. Note that not all channels linking IO and the Distributor have been initialised for you. You will need to make them and add them to the distributorChannels and ioChannels structs. These structs are created in gol/gol.go.

Step 1

You are not able to call methods directly on the IO goroutine. To use the IO, you will need to utilise channel communication. For reading in the initial PGM image, you will need the command, filename and input channels. Look at the file gol/io.go for details. The functions io.readPgmImage and startIo are particularly important in this step.

Your Game of Life code will interact with the user or the unit tests using the events channel. All events are defined in the file gol/event.go. In this step, you will only be working with the unit test TestGol. Therefore, you only need to send the FinalTurnComplete event.

Test your serial, single-threaded code using go test -v -run=TestGol/-1$. All the tests ran should pass.

Step 2
Step 2

Parallelise your Game of Life so that it uses worker threads to calculate the new state of the board. You should implement a distributor that tasks different worker threads to operate on different parts of the image in parallel. The number of worker threads you should create is specified in gol.Params.Threads.

Note: You are free to design your system as you see fit, however, we encourage you to primarily use channels

Test your code using go test -v -run=TestGol. You can use tracing to verify the correct number of workers was used this time.

Step 3
Step 3

The lab sheets included the use of a timer. Now using a ticker, report the number of cells that are still alive every 2 seconds. To report the count use the AliveCellsCount event.

Test your code using go test -v -run=TestAlive.

Step 4
Step 4

Implement logic to output the state of the board after all turns have completed as a PGM image.

Test your code using go test -v -run=TestPgm. Finally, run go test -v and make sure all tests are passing.

Step 5
Step 5

Implement logic to visualise the state of the game using SDL. You will need to use CellFlipped and TurnComplete events to achieve this. Look at sdl/loop.go for details. Don't forget to send a CellFlipped event for all initially alive cells before processing any turns.

Also, implement the following control rules. Note that the goroutine running SDL provides you with a channel containing the relevant keypresses.

If s is pressed, generate a PGM file with the current state of the board.
If q is pressed, generate a PGM file with the current state of the board and then terminate the program. Your program should not continue to execute all turns set in gol.Params.Turns.
If p is pressed, pause the processing and print the current turn that is being processed. If p is pressed again resume the processing and print "Continuing". It is not necessary for q and s to work while the execution is paused.
Test the visualisation and control rules by running go run .

Success Criteria
Pass all test cases under TestGol, TestAlive and TestPgm.
Use the correct number of workers as requested in gol.Params.
Display the live progress of the game using SDL.
Ensure that all keyboard control rules work correctly.
Use benchmarks to measure the performance of your parallel program.
The implementation must scale well with the number of worker threads.
The implementation must be free of deadlocks and race conditions.
In your Report
Discuss the goroutines you used and how they work together.
Explain and analyse the benchmark results obtained. You may want to consider using graphs to visualise your benchmarks.
Analyse how your implementation scales as more workers are added.
Briefly discuss your methodology for acquiring any results or measurements.