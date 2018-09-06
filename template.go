package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"text/template"
)

type Compilable interface {
	Compile() (string, error)
}

type importValues struct {
	Name string
	Path string
}

const importTemplate = `
{{- if ne .Name "timestamp" -}}
import * as {{.Name}} from './{{.Path}}'
{{end -}}
`

func (iv *importValues) Compile() (string, error) {
	return compileAndExecute(importTemplate, iv)
}

type enumKeyVal struct {
	Name  string
	Value int32
}

type enumValues struct {
	Name   string
	Values []*enumKeyVal
}

const enumTemplate = `
{{- $enumName := .Name}}
export const {{$enumName}} = {
  {{- range .Values}}
  {{.Name}}: {{.Value}}
  {{- end}}
}
`

func (ev *enumValues) Compile() (string, error) {
	return compileAndExecute(enumTemplate, ev)
}

type serviceValues struct {
	Name    string
	Package string
	Methods []*serviceMethodValues
}

var serviceTemplate = `
export interface {{.Name}}Interface {
  {{- range .Methods}}
  {{.Name | methodName}}: ({{.InputType | argumentName}}: {{.InputType}}) => Promise<{{.OutputType | modelName}}>
  {{- end}}
}

export class {{.Name}} implements {{.Name}}Interface {
  private hostname: string
  private fetch: Fetch
  private path = '/twirp/{{.Package}}.{{.Name}}/'

  constructor(hostname: string, fetch: Fetch) {
    this.hostname = hostname
    this.fetch = fetch
  }

  {{range .Methods}}
  {{.Name | methodName}}({{.InputType | argumentName}}: {{.InputType}}): Promise<{{.OutputType | modelName}}> {
    const url = this.hostname + this.path + '{{.Name}}'
    return this.fetch(createTwirpRequest(url, {{.InputType | modelName | typeToJSON}}({{.InputType | argumentName}}))).then((res) => {
      if (!res.ok) {
        return throwTwirpError(res)
      }
      return res.json().then({{.OutputType | jsonToType}})
    })
  }
  {{end}}
}
`

func (sv *serviceValues) Compile() (string, error) {
	return compileAndExecute(serviceTemplate, sv)
}

type serviceMethodValues struct {
	Name       string
	Path       string
	InputType  string
	OutputType string
}

type fieldValues struct {
	Name       string
	Type       string
	JSONType   string
	IsRepeated bool
}

type messageValues struct {
	Name     string
	Type     string
	JSONType string
	Fields   []*fieldValues
}

var messageTemplate = `
export interface {{.Type}} {
  {{- range .Fields}}
  {{.Name | camelCase}}: {{.Type}}
  {{- end}}
}

export interface {{.JSONType}} {
  {{- range .Fields}}
  {{.Name | jsonName}}: {{.Type | jsonType}}
  {{- end}}
}

export class {{.Name}} implements {{.Type}} {
  {{- range .Fields}}
  {{.Name | camelCase}}: {{.Type}}
  {{- end}}
}

export const {{.Type}}ToJSON = (m: {{.Type}}): {{.JSONType}} => {
  return {
    {{- range .Fields}}
    {{.Name}}: {{. | toJSON}}
    {{- end}}
  }
}

export const JSONTo{{.Name}} = (m: {{.JSONType}}): {{.Type}} => {
  return <{{.Name}}>{
    {{- range .Fields}}
    {{.Name | camelCase}}: {{. | fromJSON -}}
    {{- end}}
  }
}
`

func (mv *messageValues) Compile() (string, error) {
	return compileAndExecute(messageTemplate, mv)
}

type protoFile struct {
	Messages []*messageValues
	Services []*serviceValues
	Enums    []*enumValues
	Imports  map[string]*importValues
}

var protoTemplate = `
{{- if .Imports}}
{{- range .Imports}}
{{- . | compile}}
{{- end}}
{{- end}}

{{- if .Services}}
import {
  createTwirpRequest,
  Fetch,
  throwTwirpError
} from './twirp'
{{- end}}

{{- if .Enums}}
// Enums
{{- range .Enums}}
{{- . | compile}}
{{- end}}
{{- end}}

{{- if .Messages}}
// Messages
{{- range .Messages}}
{{- . | compile}}
{{- end}}
{{- end}}

{{- if .Services}}

// Services
{{- range .Services}}
{{- . | compile}}
{{- end}}
{{- end}}
`

func compile(c Compilable) string {
	s, err := c.Compile()
	if err != nil {
		log.Fatal("failed to compile: ", err)
	}
	return s
}

func (pf *protoFile) Compile() (string, error) {
	return compileAndExecute(protoTemplate, pf)
}

func methodName(method string) string {
	return strings.ToLower(method[0:1]) + method[1:]
}

func argumentName(method string) string {
	return "_" + methodName(removePkg(method))
}

func modelName(name string) string {
	return name + "Model"
}

func jsonName(name string) string {
	return name
}

func jsonType(name string) string {
	if strings.HasSuffix(name, "Model[]") {
		return name[0:len(name)-7] + "JSON[]"
	}
	if strings.HasSuffix(name, "Model") {
		return name[0:len(name)-5] + "JSON"
	}
	return name
}

func compileAndExecute(tpl string, data interface{}) (string, error) {
	funcMap := template.FuncMap{
		"camelCase":    camelCase,
		"compile":      compile,
		"methodName":   methodName,
		"modelName":    modelName,
		"jsonName":     jsonName,
		"jsonType":     jsonType,
		"fromJSON":     fromJSON,
		"toJSON":       toJSON,
		"jsonToType":   jsonToType,
		"typeToJSON":   typeToJSON,
		"argumentName": argumentName,
	}

	t, err := template.New("").Funcs(funcMap).Parse(tpl)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(nil)
	if err := t.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func typeToJSON(s string) string {
	return s + "ToJSON"
}

func jsonToType(typeName string) string {
	typeChunks := strings.SplitN(typeName, ".", 2)
	if len(typeChunks) > 1 {
		return fmt.Sprintf("%s.JSONTo%s", typeChunks[0], typeChunks[1])
	}
	return fmt.Sprintf("JSONTo%s", typeChunks[0])
}

func fromJSON(f fieldValues) string {
	if f.IsRepeated {
		singularType := f.Type[0 : len(f.Type)-2] // Remove []

		switch singularType {
		case "string", "number", "boolean":
			return fmt.Sprintf("(m.%s || []).map((v) => {return %s(v)})", f.Name, upperCaseFirst(singularType))
		}

		if strings.HasSuffix(singularType, "Model") {
			singularType = singularType[0 : len(singularType)-5]
		}

		return fmt.Sprintf("(m.%s || []).map(%s)", f.Name, jsonToType(singularType))
	}

	if strings.HasSuffix(f.Type, "Model") {
		return fmt.Sprintf("%s(m.%s)", jsonToType(f.Type[0:len(f.Type)-5]), f.Name)
	}

	return "m." + f.Name
}

func toJSON(f fieldValues) string {
	if f.IsRepeated {
		singularType := f.Type[0 : len(f.Type)-2] // Remove []
		switch singularType {
		case "string", "number", "boolean":
			return fmt.Sprintf("(m.%s || []).map((v) => {return %s(v)})", camelCase(f.Name), upperCaseFirst(singularType))
		}
		return fmt.Sprintf("(m.%s || []).map(%sToJSON)", camelCase(f.Name), singularType)
	}
	if strings.HasSuffix(f.Type, "Model") {
		return fmt.Sprintf("%sToJSON(m.%s)", f.Type, camelCase(f.Name))
	}
	return "m." + camelCase(f.Name)
}
