package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func closer(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Fatal(fmt.Sprintf("Error closing closer: %s", err.Error()))
	}
}

func remover(name string) {
	if err := os.Remove(name); err != nil {
		log.Fatal(fmt.Sprintf("Error removing file %s: %s", name, err.Error()))
	}
}
