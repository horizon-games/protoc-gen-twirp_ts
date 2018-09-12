package main

import (
	"errors"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type dependencyResolver struct {
	v map[string]*descriptor.FileDescriptorProto
}

func (d *dependencyResolver) Set(fd *descriptor.FileDescriptorProto, messageName string) {
	if d.v == nil {
		d.v = make(map[string]*descriptor.FileDescriptorProto)
	}
	typeName := fullTypeName(fd, messageName)
	//log.Printf("-> typeName: %v (%v)", typeName, fd.GetName())

	d.v[typeName] = fd
}

func (d *dependencyResolver) Resolve(typeName string) (*descriptor.FileDescriptorProto, error) {
	fp := d.v[typeName]
	if fp == nil {
		return nil, errors.New("no such type")
	}
	if typeName == ".google.protobuf.Timestamp" {
		return nil, errors.New("type is replaced by native Date")
	}
	return fp, nil
}

func (d *dependencyResolver) TypeName(fd *descriptor.FileDescriptorProto, typeName string) string {
	orig, err := d.Resolve(fullTypeName(fd, typeName))
	if err == nil {
		if !samePackage(fd, orig) {
			return importName(orig) + "." + typeName
		}
	}
	return typeName
}
