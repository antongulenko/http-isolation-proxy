package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/antongulenko/http-isolation-proxy/services"
)

var (
	fixKeyboard bool
	pool        *Pool
)

func main() {
	num_users := flag.Uint("users", 5, "Number of simulated people")
	orders_per_user := flag.Uint("orders", 5, "Maximum umber of simultaneous orders per person. 0 for no limitation.")
	bank := flag.String("bank", "localhost:9001", "Bank endpoint")
	timeout := flag.Duration("timeout", 0, "Timeout for automatically stopping load generation")
	dynamicUsers := flag.Bool("dynamic", false, "Enable changing # of active users with arrow keys. CTRL-C breaks console")
	var shops services.StringSlice
	flag.Var(&shops, "shop", "Shop endpoint(s)")
	flag.Parse()
	if len(shops) == 0 {
		log.Fatalln("Specify at least one -shop")
	}
	services.ConfigureOpenFilesLimit()
	pool = NewPool(*bank, shops)
	pool.OrdersPerPerson = *orders_per_user
	pool.Start(int(*num_users))

	if *dynamicUsers {
		fixKeyboard = true
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
				terminate()
			}
		})
	}
	onInterrupt(terminate)
	if *timeout > 0 {
		services.L.Warnf("Terminating automatically after %v", timeout)
		time.AfterFunc(*timeout,
			func() {
				services.L.Warnf("Timer of %v expired. Terminating...", timeout)
				terminate()
			})
	}
	pool.Wait()
	pool.PrintStats()
}

func terminate() {
	if fixKeyboard {
		resetKeyboard()
	}
	pool.Terminate()
}

func onInterrupt(handler func()) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		handler()
	}()
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
