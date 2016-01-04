package main

import (
	"flag"
	"os"
	"os/exec"
)

func main() {
	num_users := flag.Uint("users", 10, "Number of simulated people")
	bank := flag.String("bank", "localhost:9001", "Bank endpoint")
	shop := flag.String("shop", "localhost:9004", "Shop endpoint")
	flag.Parse()

	pool := NewPool(*bank, *shop)
	pool.Start(int(*num_users))
	defer resetKeyboard()
	go readKeyboard(func(b byte) {
		switch b {
		case 65: // 	Up
			pool.StartOne()
		case 66: // Down
			pool.PauseOne()
		case 67: // Right
			pool.Start(10)
		case 68: // Left
			pool.Pause(10)
		case 10: // Enter
			pool.Terminate()
		}
	})
	pool.Wait()
}

func readKeyboard(ex func(b byte)) {
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

	for {
		buf := make([]byte, 1)
		i, err := os.Stdin.Read(buf)
		if i == 1 && err == nil {
			ex(buf[0])
		}
	}
}

func resetKeyboard() {
	exec.Command("stty", "-F", "/dev/tty", "echo").Run()
}
