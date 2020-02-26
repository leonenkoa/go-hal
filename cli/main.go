package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/metal-stack/go-hal/detect"
)

var (
	band    = flag.String("bandtype", "inband", "inband/outband")
	errHelp = errors.New("usage: -bandtype inband|outband")
)

func main() {
	flag.Parse()
	switch *band {
	case "inband":
		fmt.Printf("inband test\n")
		inband()
	case "outband":
		fmt.Printf("outband test\n")
		outband()
	default:
		fmt.Printf("%s\n", errHelp)
		os.Exit(1)
	}
}

func inband() {
	board, err := detect.InBand()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Board\nVendor:%s\nName:%s\n", board.Vendor, board.Name)

	inband, err := detect.ConnectInBand()
	if err != nil {
		panic(err)
	}
	uuid, err := inband.UUID()
	if err != nil {
		panic(err)
	}
	fmt.Printf("UUID:%s\n", uuid)
}

func outband() {
	// board, err := hal.DetectOutBand()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Board:%v", board)
}