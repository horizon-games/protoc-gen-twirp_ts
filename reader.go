package main

import (
	"errors"
	"github.com/golang/protobuf/proto"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"io"
	"io/ioutil"
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
