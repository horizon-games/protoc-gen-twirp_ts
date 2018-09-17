package main

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

type packageFile struct {
	name string
	pf   []*protoFile
}

func (f *packageFile) addProto(pf *protoFile) {
	f.pf = append(f.pf, pf)
}

func (f *packageFile) protoFile() *protoFile {
	pf := &protoFile{
		Imports:  map[string]*importValues{},
		Messages: []*messageValues{},
		Services: []*serviceValues{},
		Enums:    []*enumValues{},
	}
	for i := range f.pf {
		for j := range f.pf[i].Imports {
			pf.Imports[j] = f.pf[i].Imports[j]
		}
		pf.Messages = append(pf.Messages, f.pf[i].Messages...)
		pf.Services = append(pf.Services, f.pf[i].Services...)
		pf.Enums = append(pf.Enums, f.pf[i].Enums...)
	}
	return pf
}

var (
	packageFiles = map[string]*packageFile{}
)

func addProtoToPackage(fileName string, pf *protoFile) {
	if _, ok := packageFiles[fileName]; !ok {
		packageFiles[fileName] = &packageFile{name: fileName}
	}
	packageFiles[fileName].addProto(pf)
}

func samePackage(a *descriptor.FileDescriptorProto, b *descriptor.FileDescriptorProto) bool {
	if a.GetPackage() != b.GetPackage() {
		return false
	}
	return true
}

func fullTypeName(fd *descriptor.FileDescriptorProto, typeName string) string {
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
			name := message.GetName()
			tsInterface := typeToInterface(name)
			jsonInterface := typeToJSONInterface(name)

			resolver.Set(file, name)
			resolver.Set(file, tsInterface)
			resolver.Set(file, jsonInterface)

			v := &messageValues{
				Name:          name,
				Interface:     tsInterface,
				JSONInterface: jsonInterface,

				Fields:      []*fieldValues{},
				NestedTypes: []*messageValues{},
				NestedEnums: []*enumValues{},
			}

			if len(message.GetNestedType()) > 0 {
				// TODO: add support for nested messages
				// https://developers.google.com/protocol-buffers/docs/proto#nested
				log.Printf("warning: nested messages are not supported yet")
			}

			// Add nested enums
			for _, enum := range message.GetEnumType() {
				e := &enumValues{
					Name:   fmt.Sprintf("%s_%s", message.GetName(), enum.GetName()),
					Values: []*enumKeyVal{},
				}

				for _, value := range enum.GetValue() {
					e.Values = append(e.Values, &enumKeyVal{
						Name:  value.GetName(),
						Value: value.GetNumber(),
					})
				}

				v.NestedEnums = append(v.NestedEnums, e)
			}

			// Add message fields
			for _, field := range message.GetField() {
				fp, err := resolver.Resolve(field.GetTypeName())
				if err == nil {
					if !samePackage(fp, file) {
						pfile.Imports[fp.GetPackage()] = &importValues{
							Name: importName(fp),
							Path: importPath(file, fp.GetPackage()),
						}
					}
				}

				typeName := resolver.TypeName(file, singularFieldType(message, field))

				v.Fields = append(v.Fields, &fieldValues{
					Name:  field.GetName(),
					Field: camelCase(field.GetName()),

					Type:       typeName,
					IsEnum:     field.GetType() == descriptor.FieldDescriptorProto_TYPE_ENUM,
					IsRepeated: isRepeated(field),
				})
			}

			pfile.Messages = append(pfile.Messages, v)
		}

		// Add services
		for _, service := range file.GetService() {
			resolver.Set(file, service.GetName())

			v := &serviceValues{
				Package:   file.GetPackage(),
				Name:      service.GetName(),
				Interface: typeToInterface(service.GetName()),
				Methods:   []*serviceMethodValues{},
			}

			for _, method := range service.GetMethod() {
				{
					fp, err := resolver.Resolve(method.GetInputType())
					if err == nil {
						if !samePackage(fp, file) {
							pfile.Imports[fp.GetPackage()] = &importValues{
								Name: importName(fp),
								Path: importPath(file, fp.GetPackage()),
							}
						}
					}
				}

				{
					fp, err := resolver.Resolve(method.GetOutputType())
					if err == nil {
						if !samePackage(fp, file) {
							pfile.Imports[fp.GetPackage()] = &importValues{
								Name: importName(fp),
								Path: importPath(file, fp.GetPackage()),
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

		// Add to appropriate file
		addProtoToPackage(tsFileName(file), pfile)
	}

	for packageName := range packageFiles {
		pf := packageFiles[packageName]

		// Compile to typescript
		content, err := pf.protoFile().Compile()
		if err != nil {
			log.Fatal("could not compile template: ", err)
		}

		// Add to file list
		res.File = append(res.File, &plugin.CodeGeneratorResponse_File{
			Name:    &pf.name,
			Content: &content,
		})
	}

	for i := range res.File {
		log.Printf("wrote: %v", *res.File[i].Name)
	}

	return res, nil
}

func isRepeated(field *descriptor.FieldDescriptorProto) bool {
	return field.Label != nil && *field.Label == descriptor.FieldDescriptorProto_LABEL_REPEATED
}

func removePkg(s string) string {
	p := strings.SplitN(s, ".", 3)
	c := strings.Split(p[len(p)-1], ".")
	return strings.Join(c, "_")
}

func upperCaseFirst(s string) string {
	return strings.ToUpper(s[0:1]) + s[1:]
}

func camelCase(s string) string {
	parts := strings.Split(s, "_")

	for i, p := range parts {
		if i == 0 {
			parts[i] = p
		} else {
			parts[i] = strings.ToUpper(p[0:1]) + p[1:]
		}
	}

	return strings.Join(parts, "")
}

func importName(fp *descriptor.FileDescriptorProto) string {
	return tsImportName(fp.GetPackage())
}

func tsImportName(name string) string {
	base := path.Base(name)
	return base[0 : len(base)-len(path.Ext(base))]
}

func tsImportPath(name string) string {
	base := path.Base(name)
	name = name[0 : len(name)-len(path.Ext(base))]
	return name
}

func importPath(fd *descriptor.FileDescriptorProto, name string) string {
	// TODO: how to resolve relative paths?
	return tsImportPath(name)
}

func tsFileName(fd *descriptor.FileDescriptorProto) string {
	packageName := fd.GetPackage()
	if packageName == "" {
		packageName = path.Base(fd.GetName())
	}
	name := path.Join(path.Dir(fd.GetName()), packageName)
	return tsImportPath(name) + ".ts"
}

func singularFieldType(m *descriptor.DescriptorProto, f *descriptor.FieldDescriptorProto) string {
	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE,
		descriptor.FieldDescriptorProto_TYPE_FIXED32,
		descriptor.FieldDescriptorProto_TYPE_FIXED64,
		descriptor.FieldDescriptorProto_TYPE_INT32,
		descriptor.FieldDescriptorProto_TYPE_INT64,
		descriptor.FieldDescriptorProto_TYPE_UINT32,
		descriptor.FieldDescriptorProto_TYPE_UINT64:
		return "number"
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		return removePkg(f.GetTypeName())
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		return "boolean"
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		name := f.GetTypeName()

		// Google WKT Timestamp is a special case here:
		//
		// Currently the value will just be left as jsonpb RFC 3339 string.
		// JSON.stringify already handles serializing Date to its RFC 3339 format.
		//
		if name == ".google.protobuf.Timestamp" {
			return "Date"
		}

		return removePkg(name)
	default:
		//log.Printf("unknown type %q in field %q", f.GetType(), f.GetName())
		return "string"
	}

}

func fieldType(f *fieldValues) string {
	t := f.Type
	if t == "Date" {
		t = "string"
	}
	if f.IsRepeated {
		return t + "[]"
	}
	return t
}
