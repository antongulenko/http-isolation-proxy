package main

import (
	"log"

	"github.com/antongulenko/http-isolation-proxy/services"
)

type A1 struct {
	ggg string
	ccc int

	Abc  string
	Def  float64
	Def2 int
	Def3 uint
	Def4 float32
	Def5 uint32
	Def6 uint8
	Def7 int8
	OOO  int32 `redis:"hello"`
}

type A2 struct {
	AA  A1
	BB  A1
	A   *string
	a   *string `redis:"ohoh"`
	ZZZ uint8
}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	p, err := services.ConnectRedis("localhost:6379")
	check(err)

	resp := p.Cmd("set", "ssstestlock123", "anton", "ex", 100000, "nx")
	res, _ := resp.Str()
	log.Println(resp.HasResult(), resp.Err(), res)
	resp = p.Cmd("set", "testlock", "anton", "ex", 100000, "nx")
	res, _ = resp.Str()
	log.Println(resp.HasResult(), resp.Err(), res)
}
