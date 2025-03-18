package compose

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

type CompiledGraph struct {
	run                   Runnable[any, any]
	inputType, outputType reflect.Type
}

func InvokeCompiledGraph[In, Out any](ctx context.Context, g *CompiledGraph, input In, opts ...Option) (Out, error) {
	// Type check at runtime to ensure generic types match the compiled types
	if reflect.TypeOf(input) != g.inputType {
		panic(fmt.Sprintf("input type mismatch: expected %v, got %v", g.inputType, reflect.TypeOf(input)))
	}
	if reflect.TypeOf((*Out)(nil)).Elem() != g.outputType {
		panic(fmt.Sprintf("output type mismatch: expected %v, got %v", g.outputType, reflect.TypeOf((*Out)(nil)).Elem()))
	}

	result, err := g.run.Invoke(ctx, input, opts...)
	if err != nil {
		var zero Out
		return zero, err
	}

	// Type assertion for the output
	out, ok := result.(Out)
	if !ok {
		return *new(Out), fmt.Errorf("failed to convert output to type %T", *new(Out))
	}

	return out, nil
}

func CompileGraph(ctx context.Context, dsl *GraphDSL) (*CompiledGraph, error) {
	newGraphOptions := make([]NewGraphOption, 0)
	if dsl.StateType != nil {
		stateFn := func(ctx context.Context) any {
			return dsl.StateType.New()
		}
		newGraphOptions = append(newGraphOptions, WithGenLocalState(stateFn))
	}

	g := NewGraph[any, any](newGraphOptions...)
	for _, node := range dsl.Nodes {
		if err := graphAddNode(ctx, g.graph, node); err != nil {
			return nil, err
		}
	}

	for _, edge := range dsl.Edges {
		if err := g.AddEdge(edge.From, edge.To); err != nil {
			return nil, err
		}
	}

	compileOptions := make([]GraphCompileOption, 0)
	if dsl.NodeTriggerMode != nil {
		compileOptions = append(compileOptions, WithNodeTriggerMode(*dsl.NodeTriggerMode))
	}
	if dsl.MaxRunStep != nil {
		compileOptions = append(compileOptions, WithMaxRunSteps(*dsl.MaxRunStep))
	}
	if dsl.Name != nil {
		compileOptions = append(compileOptions, WithGraphName(*dsl.Name))
	}

	r, err := g.Compile(ctx, compileOptions...)
	if err != nil {
		return nil, err
	}

	return &CompiledGraph{
		run:        r,
		inputType:  dsl.InputType.Type(),
		outputType: dsl.OutputType.Type(),
	}, nil
}

func graphAddNode(ctx context.Context, g *graph, dsl *NodeDSL) error {
	addNodeOpts := make([]GraphAddNodeOpt, 0)
	if dsl.Name != nil {
		addNodeOpts = append(addNodeOpts, WithNodeName(*dsl.Name))
	}
	if dsl.InputKey != nil {
		addNodeOpts = append(addNodeOpts, WithInputKey(*dsl.InputKey))
	}
	if dsl.OutputKey != nil {
		addNodeOpts = append(addNodeOpts, WithOutputKey(*dsl.OutputKey))
	}

	// TODO: state handlers

	implMeta, ok := implMap[dsl.ImplID]
	if !ok {
		return fmt.Errorf("impl not found: %v", dsl.ImplID)
	}

	configs := dsl.Configs

	switch implMeta.ComponentType {
	case components.ComponentOfChatModel:
	case components.ComponentOfPrompt:
		f := implMeta.InstantiationFunction
		if f == nil {
			return fmt.Errorf("instantiation function not found: %v", dsl.ImplID)
		}

		configValues := make([]reflect.Value, 0, len(configs))
		for i, conf := range configs {
			var fTypeID TypeID
			if i >= len(f.InputTypes) {
				if !f.IsVariadic {
					return fmt.Errorf("configs length mismatch: given %d, defined %d", len(configs), len(f.InputTypes))
				}
				fTypeID = f.InputTypes[len(f.InputTypes)-1]
			} else {
				fTypeID = f.InputTypes[i]
			}

			fType, ok := typeMap[fTypeID]
			if !ok {
				return fmt.Errorf("type not found: %v", fTypeID)
			}

			switch fType.InstantiationType {
			case InstantiationTypeLiteral:
				// TODO: literal
			case InstantiationTypeFunction:
				// TODO: function
			case InstantiationTypeUnmarshal:
				// TODO: reflect
				rTypePtr := fType.ReflectType
				if rTypePtr == nil {
					return fmt.Errorf("reflect type not found: %v", fTypeID)
				}
				rType := *rTypePtr
				isPtr := rType.Kind() == reflect.Ptr
				if !isPtr {
					rType = reflect.PointerTo(rType)
				}
				instance := newInstanceByType(rType).Interface()
				err := sonic.Unmarshal([]byte(conf), instance)
				if err != nil {
					return err
				}

				rValue := reflect.ValueOf(instance)
				if !isPtr {
					rValue = rValue.Elem()
				}
				configValues = append(configValues, rValue)
			default:
				return fmt.Errorf("unsupported instantiation type: %v", fType.InstantiationType)
			}
		}

		factoryFn := implMeta.InstantiationFunction.FuncValue
		results := factoryFn.Call(configValues)

		// TODO: handle the results

		addFn, ok := comp2AddFn[implMeta.ComponentType]
		if !ok {
			return fmt.Errorf("add node function not found: %v", implMeta.ComponentType)
		}
		results = addFn.Call(append([]reflect.Value{reflect.ValueOf(g), reflect.ValueOf(dsl.Key), results[0]}))
		if len(results) != 1 {
			return fmt.Errorf("add node function return value length mismatch: given %d, defined %d", len(results), 1)
		}
		if results[0].Interface() != nil {
			return results[0].Interface().(error)
		}
	case components.ComponentOfRetriever:
	case components.ComponentOfIndexer:
	case components.ComponentOfEmbedding:
	case components.ComponentOfTransformer:
	case components.ComponentOfLoader:
	case components.ComponentOfTool:
	case ComponentOfToolsNode:
	case ComponentOfGraph:
	case ComponentOfChain:
	case ComponentOfLambda:
	case ComponentOfWorkflow:
	case ComponentOfPassthrough:
		return g.AddPassthroughNode(dsl.Key, addNodeOpts...)
	default:
		return fmt.Errorf("unsupported component type: %v", implMeta.ComponentType)
	}

	return nil
}

func (t *TypeMeta) New() any {
	switch t.BasicType {
	case BasicTypeString:
		if t.IsPtr {
			return new(string)
		}
		return ""
	case BasicTypeInteger:
		if t.IntegerType == nil {
			panic(fmt.Sprintf("basic type is %v, integer type is nil", t.BasicType))
		}
		switch *t.IntegerType {
		case IntegerTypeInt8:
			if t.IsPtr {
				return new(int8)
			}
			return int8(0)
		case IntegerTypeInt16:
			if t.IsPtr {
				return new(int16)
			}
			return int16(0)
		case IntegerTypeInt32:
			if t.IsPtr {
				return new(int32)
			}
			return int32(0)
		case IntegerTypeInt64:
			if t.IsPtr {
				return new(int64)
			}
			return int64(0)
		case IntegerTypeUint8:
			if t.IsPtr {
				return new(uint8)
			}
			return uint8(0)
		case IntegerTypeUint16:
			if t.IsPtr {
				return new(uint16)
			}
			return uint16(0)
		case IntegerTypeUint32:
			if t.IsPtr {
				return new(uint32)
			}
			return uint32(0)
		case IntegerTypeUint64:
			if t.IsPtr {
				return new(uint64)
			}
			return uint64(0)
		case IntegerTypeInt:
			if t.IsPtr {
				return new(int)
			}
			return 0
		case IntegerTypeUInt:
			if t.IsPtr {
				return new(uint)
			}
			return uint(0)
		default:
			panic(fmt.Sprintf("unsupported integer type: %v", *t.IntegerType))
		}
	case BasicTypeNumber:
		if t.FloatType == nil {
			if t.IsPtr {
				return new(float64)
			}
			return float64(0)
		}
		switch *t.FloatType {
		case FloatTypeFloat32:
			if t.IsPtr {
				return new(float32)
			}
			return float32(0)
		case FloatTypeFloat64:
			if t.IsPtr {
				return new(float64)
			}
			return float64(0)
		default:
			panic(fmt.Sprintf("unsupported float type: %v", *t.FloatType))
		}
	case BasicTypeBool:
		if t.IsPtr {
			return new(bool)
		}
		return false
	case BasicTypeStruct:
		if t.StructType == nil {
			panic(fmt.Sprintf("basic type is %v, struct type is nil", t.BasicType))
		}
		switch *t.StructType {
		case StructTypeMessage:
			return &schema.Message{}
		case StructTypeDocument:
			return &schema.Document{}
		default:
			panic(fmt.Sprintf("unsupported struct type: %v", *t.StructType))
		}
	case BasicTypeArray:
		if t.ArrayType == nil {
			panic(fmt.Sprintf("basic type is %v, array type is nil", t.BasicType))
		}
		switch *t.ArrayType {
		case ArrayTypeString:
			return make([]string, 0)
		case ArrayTypeMessagePtr:
			return make([]*schema.Message, 0)
		case ArrayTypeAny:
			return make([]any, 0)
		case ArrayTypeDocumentPtr:
			return make([]*schema.Document, 0)
		default:
			panic(fmt.Sprintf("unsupported array type: %v", *t.ArrayType))
		}
	case BasicTypeMap:
		if t.MapType == nil {
			return make(map[string]any)
		}
		switch *t.MapType {
		case MapTypeStringAny:
			return make(map[string]any)
		default:
			panic(fmt.Sprintf("unsupported map type: %v", *t.MapType))
		}
	case BasicTypeInterface:
		return nil
	case BasicTypeFunction:
		return func(ctx context.Context, in any, opts ...any) (any, error) {
			return nil, nil
		}
	default:
		panic(fmt.Sprintf("unsupported basic type: %s", t.BasicType))
	}
}

func (t *TypeMeta) Type() reflect.Type {
	var typ reflect.Type

	switch t.BasicType {
	case BasicTypeString:
		if t.IsPtr {
			typ = reflect.TypeOf(new(string))
		} else {
			typ = reflect.TypeOf("")
		}
	case BasicTypeInteger:
		if t.IntegerType == nil {
			panic(fmt.Sprintf("basic type is %v, integer type is nil", t.BasicType))
		}
		switch *t.IntegerType {
		case IntegerTypeInt8:
			if t.IsPtr {
				typ = reflect.TypeOf(new(int8))
			} else {
				typ = reflect.TypeOf(int8(0))
			}
		case IntegerTypeInt16:
			if t.IsPtr {
				typ = reflect.TypeOf(new(int16))
			} else {
				typ = reflect.TypeOf(int16(0))
			}
		case IntegerTypeInt32:
			if t.IsPtr {
				typ = reflect.TypeOf(new(int32))
			} else {
				typ = reflect.TypeOf(int32(0))
			}
		case IntegerTypeInt64:
			if t.IsPtr {
				typ = reflect.TypeOf(new(int64))
			} else {
				typ = reflect.TypeOf(int64(0))
			}
		case IntegerTypeUint8:
			if t.IsPtr {
				typ = reflect.TypeOf(new(uint8))
			} else {
				typ = reflect.TypeOf(uint8(0))
			}
		case IntegerTypeUint16:
			if t.IsPtr {
				typ = reflect.TypeOf(new(uint16))
			} else {
				typ = reflect.TypeOf(uint16(0))
			}
		case IntegerTypeUint32:
			if t.IsPtr {
				typ = reflect.TypeOf(new(uint32))
			} else {
				typ = reflect.TypeOf(uint32(0))
			}
		case IntegerTypeUint64:
			if t.IsPtr {
				typ = reflect.TypeOf(new(uint64))
			} else {
				typ = reflect.TypeOf(uint64(0))
			}
		case IntegerTypeInt:
			if t.IsPtr {
				typ = reflect.TypeOf(new(int))
			} else {
				typ = reflect.TypeOf(int(0))
			}
		case IntegerTypeUInt:
			if t.IsPtr {
				typ = reflect.TypeOf(new(uint))
			} else {
				typ = reflect.TypeOf(uint(0))
			}
		default:
			panic(fmt.Sprintf("unsupported integer type: %v", *t.IntegerType))
		}
	case BasicTypeNumber:
		if t.FloatType == nil {
			if t.IsPtr {
				typ = reflect.TypeOf(new(float64))
			} else {
				typ = reflect.TypeOf(float64(0))
			}
		} else {
			switch *t.FloatType {
			case FloatTypeFloat32:
				if t.IsPtr {
					typ = reflect.TypeOf(new(float32))
				} else {
					typ = reflect.TypeOf(float32(0))
				}
			case FloatTypeFloat64:
				if t.IsPtr {
					typ = reflect.TypeOf(new(float64))
				} else {
					typ = reflect.TypeOf(float64(0))
				}
			default:
				panic(fmt.Sprintf("unsupported float type: %v", *t.FloatType))
			}
		}
	case BasicTypeBool:
		if t.IsPtr {
			typ = reflect.TypeOf(new(bool))
		} else {
			typ = reflect.TypeOf(false)
		}
	case BasicTypeStruct:
		if t.StructType == nil {
			panic(fmt.Sprintf("basic type is %v, struct type is nil", t.BasicType))
		}
		switch *t.StructType {
		case StructTypeMessage:
			if t.IsPtr {
				typ = reflect.TypeOf(&schema.Message{})
			} else {
				typ = reflect.TypeOf(schema.Message{})
			}
		case StructTypeDocument:
			if t.IsPtr {
				typ = reflect.TypeOf(&schema.Document{})
			} else {
				typ = reflect.TypeOf(schema.Document{})
			}
		default:
			panic(fmt.Sprintf("unsupported struct type: %v", *t.StructType))
		}
	case BasicTypeArray:
		if t.ArrayType == nil {
			panic(fmt.Sprintf("basic type is %v, array type is nil", t.BasicType))
		}
		var sliceType reflect.Type
		switch *t.ArrayType {
		case ArrayTypeString:
			sliceType = reflect.TypeOf([]string{})
		case ArrayTypeMessagePtr:
			sliceType = reflect.TypeOf([]*schema.Message{})
		case ArrayTypeAny:
			sliceType = reflect.TypeOf([]any{})
		case ArrayTypeDocumentPtr:
			sliceType = reflect.TypeOf([]*schema.Document{})
		default:
			panic(fmt.Sprintf("unsupported array type: %v", *t.ArrayType))
		}
		if t.IsPtr {
			typ = reflect.PointerTo(sliceType)
		} else {
			typ = sliceType
		}
	case BasicTypeMap:
		var mapType reflect.Type
		if t.MapType == nil {
			mapType = reflect.TypeOf(map[string]any{})
		} else {
			switch *t.MapType {
			case MapTypeStringAny:
				mapType = reflect.TypeOf(map[string]any{})
			default:
				panic(fmt.Sprintf("unsupported map type: %v", *t.MapType))
			}
		}
		if t.IsPtr {
			typ = reflect.PointerTo(mapType)
		} else {
			typ = mapType
		}
	case BasicTypeInterface:
		if t.IsPtr {
			typ = reflect.TypeOf((*any)(nil))
		} else {
			typ = reflect.TypeOf((*any)(nil)).Elem()
		}
	case BasicTypeFunction:
		fnType := reflect.TypeOf(func(context.Context, any, ...any) (any, error) { return nil, nil })
		if t.IsPtr {
			typ = reflect.PointerTo(fnType)
		} else {
			typ = fnType
		}
	default:
		panic(fmt.Sprintf("unsupported basic type: %s", t.BasicType))
	}

	return typ
}
