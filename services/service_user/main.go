package main

import (
	"flag"
	"log"
	"os"
	"os/exec"

	"github.com/antongulenko/http-isolation-proxy/services"
)

func main() {
	num_users := flag.Uint("users", 5, "Number of simulated people")
	bank := flag.String("bank", "localhost:9001", "Bank endpoint")
	var shops services.StringSlice
	flag.Var(&shops, "shop", "Shop endpoint(s)")
	flag.Parse()
	if len(shops) == 0 {
		log.Fatalln("Specify at least one -shop")
	}
	pool := NewPool(*bank, shops)
	pool.Start(int(*num_users))

	defer resetKeyboard()
	go readKeyboard(func(b byte) {
		switch b {
		case 65: // Up
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
