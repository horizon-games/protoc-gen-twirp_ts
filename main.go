package main

import (
	"log"
	"os"

	"errors"
	"io"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func read(rdr io.Reader) (*plugin.CodeGeneratorRequest, error) {
	buf, err := ioutil.ReadAll(rdr)
	if err != nil {
		return nil, err
	}

	req := &plugin.CodeGeneratorRequest{}

	if err := proto.Unmarshal(buf, req); err != nil {
		return nil, err
	}

	if len(req.FileToGenerate) < 1 {
		return nil, errors.New(`no files to generate`)
	}

	return req, err
}

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
