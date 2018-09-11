package main

import (
	"log"
	"strings"
)

type compilable interface {
	Compile() (string, error)
}

func compile(c compilable) string {
	s, err := c.Compile()
	if err != nil {
		log.Fatal("failed to compile: ", err)
	}
	return strings.TrimSpace(s)
}
