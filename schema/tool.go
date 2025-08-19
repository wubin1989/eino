/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package schema

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// DataType is the type of the parameter.
// It must be one of the following values: "object", "number", "integer", "string", "array", "null", "boolean", which is the same as the type of the parameter in OpenAPI v3.0.
type DataType string

const (
	Object  DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

// ToolChoice controls how the model calls tools (if any).
type ToolChoice string

const (
	// ToolChoiceForbidden indicates that the model should not call any tools.
	// Corresponds to "none" in OpenAI Chat Completion.
	ToolChoiceForbidden ToolChoice = "forbidden"

	// ToolChoiceAllowed indicates that the model can choose to generate a message or call one or more tools.
	// Corresponds to "auto" in OpenAI Chat Completion.
	ToolChoiceAllowed ToolChoice = "allowed"

	// ToolChoiceForced indicates that the model must call one or more tools.
	// Corresponds to "required" in OpenAI Chat Completion.
	ToolChoiceForced ToolChoice = "forced"
)

// ToolInfo is the information of a tool.
type ToolInfo struct {
	// The unique name of the tool that clearly communicates its purpose.
	Name string
	// Used to tell the model how/when/why to use the tool.
	// You can provide few-shot examples as a part of the description.
	Desc string
	// Extra is the extra information for the tool.
	Extra map[string]any

	// The parameters the functions accepts (different models may require different parameter types).
	// can be described in two ways:
	//  - use params: schema.NewParamsOneOfByParams(params)
	//  - use openAPIV3: schema.NewParamsOneOfByOpenAPIV3(openAPIV3)
	// If is nil, signals that the tool does not need any input parameter
	*ParamsOneOf
}

// ParameterInfo is the information of a parameter.
// It is used to describe the parameters of a tool.
type ParameterInfo struct {
	// The type of the parameter.
	Type DataType
	// The element type of the parameter, only for array.
	ElemInfo *ParameterInfo
	// The sub parameters of the parameter, only for object.
	SubParams map[string]*ParameterInfo
	// The description of the parameter.
	Desc string
	// The enum values of the parameter, only for string.
	Enum []string
	// Whether the parameter is required.
	Required bool
}

// ParamsOneOf is a union of the different methods user can choose which describe a tool's request parameters.
// User must specify one and ONLY one method to describe the parameters.
//  1. use NewParamsOneOfByParams(): an intuitive way to describe the parameters that covers most of the use-cases.
//  2. use NewParamsOneOfByOpenAPIV3(): a formal way to describe the parameters that strictly adheres to OpenAPIV3.0 specification.
//     See https://github.com/getkin/kin-openapi/blob/master/openapi3/schema.go.
type ParamsOneOf struct {
	// use NewParamsOneOfByParams to set this field
	params map[string]*ParameterInfo

	// use NewParamsOneOfByOpenAPIV3 to set this field
	openAPIV3 *openapi3.Schema

	jsonschema *jsonschema.Schema
}

// NewParamsOneOfByParams creates a ParamsOneOf with map[string]*ParameterInfo.
func NewParamsOneOfByParams(params map[string]*ParameterInfo) *ParamsOneOf {
	return &ParamsOneOf{
		params: params,
	}
}

// NewParamsOneOfByJSONSchema creates a ParamsOneOf with *jsonschema.Schema.
func NewParamsOneOfByJSONSchema(s *jsonschema.Schema) *ParamsOneOf {
	return &ParamsOneOf{
		jsonschema: s,
	}
}

// Deprecated: use NewParamsOneOfByJSONSchema instead.
// For more information, see https://github.com/cloudwego/eino/discussions/397.
// NewParamsOneOfByOpenAPIV3 creates a ParamsOneOf with *openapi3.Schema.
func NewParamsOneOfByOpenAPIV3(openAPIV3 *openapi3.Schema) *ParamsOneOf {
	return &ParamsOneOf{
		openAPIV3: openAPIV3,
	}
}

// Deprecated: use ToJSONSchema instead.
// For more information, see https://github.com/cloudwego/eino/discussions/397.
// ToOpenAPIV3 parses ParamsOneOf, converts the parameter description that user actually provides, into the format ready to be passed to Model.
func (p *ParamsOneOf) ToOpenAPIV3() (*openapi3.Schema, error) {
	if p == nil {
		return nil, nil
	}

	if p.params != nil {
		sc := &openapi3.Schema{
			Properties: make(map[string]*openapi3.SchemaRef, len(p.params)),
			Type:       openapi3.TypeObject,
			Required:   make([]string, 0, len(p.params)),
		}

		for k := range p.params {
			v := p.params[k]
			sc.Properties[k] = paramInfoToOpenAPIV3(v)
			if v.Required {
				sc.Required = append(sc.Required, k)
			}
		}

		return sc, nil
	}

	if p.jsonschema != nil {
		sc := jsonSchemaToOpenAPIV3(p.jsonschema)
		return sc.Value, nil
	}

	return p.openAPIV3, nil
}

// ToJSONSchema parses ParamsOneOf, converts the parameter description that user actually provides, into the format ready to be passed to Model.
func (p *ParamsOneOf) ToJSONSchema() (*jsonschema.Schema, error) {
	if p == nil {
		return nil, nil
	}

	if p.params != nil {
		sc := &jsonschema.Schema{
			Properties: orderedmap.New[string, *jsonschema.Schema](),
			Type:       openapi3.TypeObject,
			Required:   make([]string, 0, len(p.params)),
		}

		for k := range p.params {
			v := p.params[k]
			sc.Properties.Set(k, paramInfoToJSONSchema(v))
			if v.Required {
				sc.Required = append(sc.Required, k)
			}
		}

		return sc, nil
	}

	return p.jsonschema, nil
}

func paramInfoToOpenAPIV3(paramInfo *ParameterInfo) *openapi3.SchemaRef {
	var types string
	switch paramInfo.Type {
	case Null:
		types = "null"
	case Boolean:
		types = openapi3.TypeBoolean
	case Integer:
		types = openapi3.TypeInteger
	case Number:
		types = openapi3.TypeNumber
	case String:
		types = openapi3.TypeString
	case Array:
		types = openapi3.TypeArray
	case Object:
		types = openapi3.TypeObject
	}

	js := &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type:        types,
			Description: paramInfo.Desc,
		},
	}

	if len(paramInfo.Enum) > 0 {
		js.Value.Enum = make([]any, 0, len(paramInfo.Enum))
		for _, enum := range paramInfo.Enum {
			js.Value.Enum = append(js.Value.Enum, enum)
		}
	}

	if paramInfo.ElemInfo != nil {
		js.Value.Items = paramInfoToOpenAPIV3(paramInfo.ElemInfo)
	}

	if len(paramInfo.SubParams) > 0 {
		required := make([]string, 0, len(paramInfo.SubParams))
		js.Value.Properties = make(map[string]*openapi3.SchemaRef, len(paramInfo.SubParams))
		for k, v := range paramInfo.SubParams {
			item := paramInfoToOpenAPIV3(v)

			js.Value.Properties[k] = item

			if v.Required {
				required = append(required, k)
			}
		}

		js.Value.Required = required
	}

	return js
}

func paramInfoToJSONSchema(paramInfo *ParameterInfo) *jsonschema.Schema {
	var types string
	switch paramInfo.Type {
	case Null:
		types = "null"
	case Boolean:
		types = "boolean"
	case Integer:
		types = "integer"
	case Number:
		types = "number"
	case String:
		types = "string"
	case Array:
		types = "array"
	case Object:
		types = "object"
	}

	js := &jsonschema.Schema{
		Type:        types,
		Description: paramInfo.Desc,
	}

	if len(paramInfo.Enum) > 0 {
		js.Enum = make([]any, len(paramInfo.Enum))
		for i, enum := range paramInfo.Enum {
			js.Enum[i] = enum
		}
	}

	if paramInfo.ElemInfo != nil {
		js.Items = paramInfoToJSONSchema(paramInfo.ElemInfo)
	}

	if len(paramInfo.SubParams) > 0 {
		required := make([]string, 0, len(paramInfo.SubParams))
		js.Properties = orderedmap.New[string, *jsonschema.Schema]()
		for k, v := range paramInfo.SubParams {
			item := paramInfoToJSONSchema(v)
			js.Properties.Set(k, item)
			if v.Required {
				required = append(required, k)
			}
		}

		js.Required = required
	}

	return js
}

func jsonSchemaToOpenAPIV3(js *jsonschema.Schema) *openapi3.SchemaRef {
	s := openapi3.NewSchema()
	s.Type = js.Type
	s.Description = js.Description

	if js.Enum != nil {
		s.Enum = make([]any, len(js.Enum))
		for i, enum := range js.Enum {
			s.Enum[i] = enum
		}
	}

	if js.Items != nil {
		s.Items = jsonSchemaToOpenAPIV3(js.Items)
	}

	if js.Properties != nil {
		s.Properties = make(map[string]*openapi3.SchemaRef, js.Properties.Len())
		for pair := js.Properties.Oldest(); pair != nil; pair = pair.Next() {
			s.Properties[pair.Key] = jsonSchemaToOpenAPIV3(pair.Value)
		}
	}

	if js.Required != nil {
		s.Required = make([]string, len(js.Required))
		for i, required := range js.Required {
			s.Required[i] = required
		}
	}

	return openapi3.NewSchemaRef("", s)
}
