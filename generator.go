package main

import (
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"log"
	"path"
	"strings"
)

func generate(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	res := &plugin.CodeGeneratorResponse{
		File: []*plugin.CodeGeneratorResponse_File{},
	}

	for _, file := range req.GetProtoFile() {
		pfile := &protoFile{
			Messages: []*messageValues{},
			Services: []*serviceValues{},
		}

		// Add messages
		for _, message := range file.GetMessageType() {
			v := &messageValues{
				Name:     message.GetName(),
				Type:     message.GetName() + "Model",
				JSONType: message.GetName() + "JSON",
				Fields:   []*fieldValues{},
			}
			for _, field := range message.GetField() {
				tsType, jsonType := protoToTSType(field)
				v.Fields = append(v.Fields, &fieldValues{
					Name:       field.GetName(),
					Type:       tsType,
					JSONType:   jsonType,
					IsRepeated: isRepeated(field),
				})
			}
			pfile.Messages = append(pfile.Messages, v)
		}

		// Add services
		for _, service := range file.GetService() {
			v := &serviceValues{
				Name:    service.GetName(),
				Methods: []*serviceMethodValues{},
			}
			for _, method := range service.GetMethod() {
				v.Methods = append(v.Methods, &serviceMethodValues{
					Name:       method.GetName(),
					InputType:  method.GetInputType(),
					OutputType: method.GetOutputType(),
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
			tsType = removePkg(name)
			jsonType = removePkg(name) + "JSON"
		}
	}

	if isRepeated(f) {
		tsType = tsType + "Model[]"
		jsonType = jsonType + "JSON[]"
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

func tsFileName(name string) string {
	base := path.Base(name)
	name = name[:len(base)-len(path.Ext(base))]

	return name + ".ts"
}
