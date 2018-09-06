package main

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"log"
	"path"
	"strings"
)

type dependencyResolver struct {
	v map[string]*descriptor.FileDescriptorProto
}

func samePackage(a *descriptor.FileDescriptorProto, b *descriptor.FileDescriptorProto) bool {
	if a.GetPackage() != b.GetPackage() {
		return false
	}
	if a.GetName() != b.GetName() {
		return false
	}
	return true
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

func fullTypeName(fd *descriptor.FileDescriptorProto, typeName string) string {
	if strings.HasSuffix(typeName, "[]") {
		typeName = typeName[0 : len(typeName)-2]
	}
	return fmt.Sprintf(".%s.%s", fd.GetPackage(), typeName)
}

func generate(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	resolver := dependencyResolver{}

	res := &plugin.CodeGeneratorResponse{
		File: []*plugin.CodeGeneratorResponse_File{
			{
				Name:    &twirpFileName,
				Content: &twirpSource,
			},
		},
	}

	protoFiles := req.GetProtoFile()
	for i := range protoFiles {
		file := protoFiles[i]

		pfile := &protoFile{
			Imports:  map[string]*importValues{},
			Messages: []*messageValues{},
			Services: []*serviceValues{},
			Enums:    []*enumValues{},
		}

		// Add enum
		for _, enum := range file.GetEnumType() {
			resolver.Set(file, enum.GetName())

			v := &enumValues{
				Name:   enum.GetName(),
				Values: []*enumKeyVal{},
			}

			for _, value := range enum.GetValue() {
				v.Values = append(v.Values, &enumKeyVal{
					Name:  value.GetName(),
					Value: value.GetNumber(),
				})
			}

			pfile.Enums = append(pfile.Enums, v)
		}

		// Add messages
		for _, message := range file.GetMessageType() {
			resolver.Set(file, message.GetName())
			resolver.Set(file, message.GetName()+"Model")
			resolver.Set(file, message.GetName()+"JSON")

			v := &messageValues{
				Name:     message.GetName(),
				Type:     message.GetName() + "Model",
				JSONType: message.GetName() + "JSON",
				Fields:   []*fieldValues{},
			}

			for _, field := range message.GetField() {
				//log.Printf("field.type: %v", field.GetTypeName())

				fp, err := resolver.Resolve(field.GetTypeName())
				if err == nil {
					if !samePackage(fp, file) {
						pfile.Imports[fp.GetName()] = &importValues{
							Name: importName(fp),
							Path: importPath(fp.GetName()),
						}
					}
				}

				tsType, jsonType := protoToTSType(field)
				//log.Printf("tsType: %v, fullTsType: %v", tsType, resolver.TypeName(file, tsType))
				//log.Printf("jsonType: %v, fullJSONType: %v", jsonType, resolver.TypeName(file, jsonType))

				v.Fields = append(v.Fields, &fieldValues{
					Name:       field.GetName(),
					Type:       resolver.TypeName(file, tsType),
					JSONType:   resolver.TypeName(file, jsonType),
					IsRepeated: isRepeated(field),
				})
			}

			pfile.Messages = append(pfile.Messages, v)
		}

		// Add services
		for _, service := range file.GetService() {
			resolver.Set(file, service.GetName())

			v := &serviceValues{
				Package: file.GetPackage(),
				Name:    service.GetName(),
				Methods: []*serviceMethodValues{},
			}

			for _, method := range service.GetMethod() {
				{
					fp, err := resolver.Resolve(method.GetInputType())
					if err == nil {
						if !samePackage(fp, file) {
							pfile.Imports[fp.GetName()] = &importValues{
								Name: importName(fp),
								Path: importPath(fp.GetName()),
							}
						}
					}
				}

				{
					fp, err := resolver.Resolve(method.GetOutputType())
					if err == nil {
						if !samePackage(fp, file) {
							pfile.Imports[fp.GetName()] = &importValues{
								Name: importName(fp),
								Path: importPath(fp.GetName()),
							}
						}
					}
				}

				v.Methods = append(v.Methods, &serviceMethodValues{
					Name:       method.GetName(),
					InputType:  resolver.TypeName(file, removePkg(method.GetInputType())),
					OutputType: resolver.TypeName(file, removePkg(method.GetOutputType())),
				})
			}

			pfile.Services = append(pfile.Services, v)
		}

		// Compile to typescript
		s, err := pfile.Compile()
		if err != nil {
			log.Fatal("could not compile template: ", err)
		}

		fileName := tsFileName(file.GetName())
		log.Printf("fileName: %v", fileName)

		res.File = append(res.File, &plugin.CodeGeneratorResponse_File{
			Name:    &fileName,
			Content: &s,
		})
	}

	return res, nil
}

// generates the (Type, JSONType) tuple for a ModelField so marshal/unmarshal functions
// will work when converting between TS interfaces and protobuf JSON.
func protoToTSType(f *descriptor.FieldDescriptorProto) (string, string) {
	// From https://github.com/larrymyers/protoc-gen-twirp_typescript/blob/master/generator/client.go
	tsType := "string"
	jsonType := "string"

	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE,
		descriptor.FieldDescriptorProto_TYPE_FIXED32,
		descriptor.FieldDescriptorProto_TYPE_FIXED64,
		descriptor.FieldDescriptorProto_TYPE_INT32,
		descriptor.FieldDescriptorProto_TYPE_INT64:
		tsType = "number"
		jsonType = "number"
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		tsType = "string"
		jsonType = "string"
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		tsType = "boolean"
		jsonType = "boolean"
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		name := f.GetTypeName()

		// Google WKT Timestamp is a special case here:
		//
		// Currently the value will just be left as jsonpb RFC 3339 string.
		// JSON.stringify already handles serializing Date to its RFC 3339 format.
		//
		if name == ".google.protobuf.Timestamp" {
			tsType = "Date"
			jsonType = "string"
		} else {
			tsType = removePkg(name) + "Model"
			jsonType = removePkg(name) + "JSON"
		}
	}

	if isRepeated(f) {
		tsType = tsType + "[]"
		jsonType = jsonType + "[]"
	}

	return tsType, jsonType
}

func isRepeated(field *descriptor.FieldDescriptorProto) bool {
	return field.Label != nil && *field.Label == descriptor.FieldDescriptorProto_LABEL_REPEATED
}

func removePkg(s string) string {
	p := strings.Split(s, ".")
	return p[len(p)-1]
}

func upperCaseFirst(s string) string {
	return strings.ToUpper(s[0:1]) + strings.ToLower(s[1:])
}

func camelCase(s string) string {
	parts := strings.Split(s, "_")

	for i, p := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(p)
		} else {
			parts[i] = strings.ToUpper(p[0:1]) + strings.ToLower(p[1:])
		}
	}

	return strings.Join(parts, "")
}

func importName(fp *descriptor.FileDescriptorProto) string {
	name := fp.GetName()
	base := path.Base(name)
	return base[0 : len(base)-len(path.Ext(base))]
}

func importPath(name string) string {
	base := path.Base(name)

	name = name[0 : len(name)-len(path.Ext(base))]

	return name
}

func tsFileName(name string) string {
	return importPath(name) + ".ts"
}
