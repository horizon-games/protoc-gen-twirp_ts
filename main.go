package main

import (
	"github.com/golang/protobuf/proto"
	"log"
	"os"
)

func main() {
	req, err := read(os.Stdin)
	if err != nil {
		log.Fatal("read: ", err)
	}

	res, err := generate(req)
	if err != nil {
		log.Fatal("generate: ", err)
	}

	buf, err := proto.Marshal(res)
	if err != nil {
		log.Fatal("marshal: ", err)
	}

	os.Stdout.Write(buf)
}
