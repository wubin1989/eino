package compose

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/components"
)

type CompiledGraph struct {
	run Runnable[any, any]
}

func InvokeCompiledGraph[In, Out any](ctx context.Context, g *CompiledGraph, input In, opts ...Option) (Out, error) {
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
			return dsl.StateType.InstantiateByLiteral()
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
		run: r,
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

	if implMeta.ComponentType == ComponentOfPassthrough {
		return g.AddPassthroughNode(dsl.Key, addNodeOpts...)
	}

	typeMeta, ok := typeMap[implMeta.TypeID]
	if !ok {
		return fmt.Errorf("type not found: %v", implMeta.TypeID)
	}

	switch implMeta.ComponentType {
	case components.ComponentOfChatModel:
	case components.ComponentOfPrompt:
		result, err := typeMeta.Instantiate(ctx, dsl.Config, dsl.Configs, dsl.Slots)
		if err != nil {
			return err
		}

		addFn, ok := comp2AddFn[implMeta.ComponentType]
		if !ok {
			return fmt.Errorf("add node function not found: %v", implMeta.ComponentType)
		}
		results := addFn.Call(append([]reflect.Value{reflect.ValueOf(g), reflect.ValueOf(dsl.Key), result}))
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
	default:
		return fmt.Errorf("unsupported component type: %v", implMeta.ComponentType)
	}

	return nil
}

// pathSegment represents a parsed segment of a JSONPath
type pathSegment struct {
	field    string // field or map key name
	arrayIdx int    // array index if present, -1 if not
	isArray  bool   // whether this segment contains an array index
}

// parsePathSegment parses a path segment and returns its components
func parsePathSegment(segment string) (pathSegment, error) {
	result := pathSegment{arrayIdx: -1}

	if idx := strings.Index(segment, "["); idx != -1 {
		if !strings.HasSuffix(segment, "]") {
			return result, fmt.Errorf("invalid array syntax in segment: %s", segment)
		}
		result.isArray = true
		result.field = segment[:idx]

		// Parse array index
		idxStr := segment[idx+1 : len(segment)-1]
		index, err := strconv.Atoi(idxStr)
		if err != nil {
			return result, fmt.Errorf("invalid array index at %s: %v", segment, err)
		}
		result.arrayIdx = index
	} else {
		result.field = segment
	}

	return result, nil
}

// dereferencePointer follows pointer chain until a non-pointer value is found
func dereferencePointer(current reflect.Value) reflect.Value {
	for current.Kind() == reflect.Ptr {
		if current.IsNil() {
			current.Set(reflect.New(current.Type().Elem()))
		}
		current = current.Elem()
	}
	return current
}

// handleMapAccess handles access to map fields and creates/updates values as needed
func handleMapAccess(current reflect.Value, key string, isLast bool, value reflect.Value) (reflect.Value, error) {
	if current.IsNil() {
		current.Set(reflect.MakeMap(current.Type()))
	}

	if isLast {
		if current.Type().Elem().Kind() == reflect.Ptr {
			// For pointer map values, create a new pointer and set its value
			ptr := reflect.New(current.Type().Elem().Elem())
			if value.Kind() == reflect.Ptr && !value.IsNil() {
				ptr.Elem().Set(value.Elem())
			} else {
				ptr.Elem().Set(value)
			}
			current.SetMapIndex(reflect.ValueOf(key), ptr)
		} else {
			current.SetMapIndex(reflect.ValueOf(key), value)
		}
		return reflect.Value{}, nil
	}

	fieldValue := current.MapIndex(reflect.ValueOf(key))
	if !fieldValue.IsValid() {
		// Create new value based on the map's element type
		if current.Type().Elem().Kind() == reflect.Ptr {
			// For pointer types, create a new pointer and set it in the map
			newValue := reflect.New(current.Type().Elem().Elem())
			current.SetMapIndex(reflect.ValueOf(key), newValue)
			return newValue.Elem(), nil
		} else {
			before := current.Interface()
			_ = before
			// For non-pointer types, create a new value
			newValue := reflect.New(current.Type().Elem()).Elem()
			current.SetMapIndex(reflect.ValueOf(key), newValue)
			// Return a copy that we can modify
			after := current.Interface()
			_ = after
			return current.MapIndex(reflect.ValueOf(key)), nil
		}
	}

	// If the map value is a pointer, dereference it for modification
	if current.Type().Elem().Kind() == reflect.Ptr {
		if fieldValue.IsNil() {
			newValue := reflect.New(current.Type().Elem().Elem())
			current.SetMapIndex(reflect.ValueOf(key), newValue)
			return newValue.Elem(), nil
		}
		return fieldValue.Elem(), nil
	}

	// For non-pointer map values, create a new value to modify
	// This is crucial - we need a settable copy of the map value
	newValue := reflect.New(current.Type().Elem()).Elem()
	newValue.Set(fieldValue)
	// We'll update the map with this value in setValue later
	current.SetMapIndex(reflect.ValueOf(key), newValue)
	return newValue, nil
}

// handleArrayAccess handles access to array/slice elements
func handleArrayAccess(current reflect.Value, index int, isLast bool, value reflect.Value) (reflect.Value, error) {
	if !current.IsValid() || current.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("expected slice, got %v", current.Kind())
	}

	if current.IsNil() {
		current.Set(reflect.MakeSlice(current.Type(), 0, 0))
	}

	// Grow slice if needed
	for current.Len() <= index {
		current.Set(reflect.Append(current, reflect.Zero(current.Type().Elem())))
	}

	if isLast {
		return reflect.Value{}, setValue(current.Index(index), value)
	}

	return current.Index(index), nil
}

// setFieldByJSONPath sets a field in the target struct using a JSONPath
func setFieldByJSONPath(target reflect.Value, path string, value reflect.Value) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	// Remove the leading $ if present
	if path[0] == '$' {
		path = path[1:]
	}

	// Split the path into segments
	segments := strings.Split(strings.Trim(path, "."), ".")
	current := target

	// Keep track of parent values and their keys/indices for updating maps and slices
	type parentNode struct {
		value   reflect.Value
		key     reflect.Value // For maps
		index   int           // For slices/arrays
		isMap   bool
		isSlice bool
	}

	var parents []parentNode

	// Navigate through the path
	for i, segmentStr := range segments {
		targetInterface := target.Interface()
		_ = targetInterface

		isLast := i == len(segments)-1

		// Dereference any pointers
		current = dereferencePointer(current)

		// Parse the current segment
		segment, err := parsePathSegment(segmentStr)
		if err != nil {
			return err
		}

		// Handle field access if present
		if segment.field != "" {
			if current.Kind() != reflect.Struct && current.Kind() != reflect.Map {
				return fmt.Errorf("invalid path: expected struct or map at %s, got %v",
					strings.Join(segments[:i], "."), current.Kind())
			}

			if current.Kind() == reflect.Map {
				// Store the current map as a parent before we descend
				parents = append(parents, parentNode{
					value: current,
					key:   reflect.ValueOf(segment.field),
					isMap: true,
				})

				var err error
				current, err = handleMapAccess(current, segment.field, isLast && !segment.isArray, value)
				if err != nil {
					return err
				}
				if isLast && !segment.isArray {
					return nil
				}
			} else {
				// For struct fields, we don't need to track parent because
				// structs are passed by reference when modifying their fields
				current = current.FieldByName(segment.field)
				if !current.IsValid() {
					return fmt.Errorf("field not found: %s", segment.field)
				}
				if isLast && !segment.isArray {
					return setValue(current, value)
				}
			}
		}

		// Handle array access if present
		if segment.isArray {
			// Store the current slice as a parent before we descend
			parents = append(parents, parentNode{
				value:   current,
				index:   segment.arrayIdx,
				isSlice: true,
			})

			var err error
			current, err = handleArrayAccess(current, segment.arrayIdx, isLast, value)
			if err != nil {
				return err
			}
			if isLast {
				return nil
			}
		}
	}

	return nil
}

func (t *TypeMeta) InstantiateByUnmarshal(ctx context.Context, value string, slots []Slot) (reflect.Value, error) {
	rTypePtr := t.ReflectType
	if rTypePtr == nil {
		return reflect.Value{}, fmt.Errorf("reflect type not found: %v", t.ID)
	}
	rType := *rTypePtr
	isPtr := rType.Kind() == reflect.Ptr
	if !isPtr {
		rType = reflect.PointerTo(rType)
	}
	instance := newInstanceByType(rType).Interface()
	err := sonic.Unmarshal([]byte(value), instance)
	if err != nil {
		return reflect.Value{}, err
	}

	rValue := reflect.ValueOf(instance)
	if !isPtr {
		rValue = rValue.Elem()
	}

	for _, slot := range slots {
		slotType, ok := typeMap[slot.TypeID]
		if !ok {
			return reflect.Value{}, fmt.Errorf("type not found: %v", slot.TypeID)
		}

		slotInstance, err := slotType.Instantiate(ctx, slot.Config, slot.Configs, slot.Slots)
		if err != nil {
			return reflect.Value{}, err
		}

		// Parse the JSONPath and set the field
		if err := setFieldByJSONPath(rValue, slot.Path, slotInstance); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to set field by path %s: %v", slot.Path, err)
		}
	}

	return rValue, nil
}

func (t *TypeMeta) InstantiateByFunction(ctx context.Context, config *string, configs []Config, slots []Slot) (reflect.Value, error) {
	if config != nil && len(configs) > 0 {
		return reflect.Value{}, fmt.Errorf("config and configs cannot be both set")
	}

	fMeta := t.FunctionMeta
	if fMeta == nil {
		return reflect.Value{}, fmt.Errorf("function meta not found: %v", t.ID)
	}

	var (
		results          []reflect.Value
		nonCtxParamCount = len(fMeta.InputTypes)
	)
	if len(fMeta.InputTypes) == 0 {
		results = fMeta.FuncValue.Call([]reflect.Value{})
	} else {
		inputs := make(map[int]reflect.Value, len(configs)+1)
		first := fMeta.InputTypes[0]
		if first == ctxType.ID {
			nonCtxParamCount--
			inputs[0] = reflect.ValueOf(ctx)
			if config != nil {
				configs = []Config{{
					Index: 1,
					Value: *config,
				}}
			}
		} else {
			if config != nil {
				configs = []Config{
					{
						Index: 0,
						Value: *config,
					},
				}
			}
		}

		for _, conf := range configs {
			var fTypeID TypeID
			if conf.Index >= nonCtxParamCount {
				if !fMeta.IsVariadic {
					return reflect.Value{}, fmt.Errorf("configs length mismatch: given %d, nonCtxParamCount %d", len(configs), nonCtxParamCount)
				}
				fTypeID = fMeta.InputTypes[len(fMeta.InputTypes)-1]
			} else {
				fTypeID = fMeta.InputTypes[conf.Index]
			}

			fType, ok := typeMap[fTypeID]
			if !ok {
				return reflect.Value{}, fmt.Errorf("type not found: %v", fTypeID)
			}

			switch fType.InstantiationType {
			case InstantiationTypeUnmarshal:
				rValue, err := fType.InstantiateByUnmarshal(ctx, conf.Value, slots)
				if err != nil {
					return reflect.Value{}, err
				}
				if _, ok := inputs[conf.Index]; ok {
					return reflect.Value{}, fmt.Errorf("duplicate config index: %d", conf.Index)
				}

				inputs[conf.Index] = rValue
			default:
				return reflect.Value{}, fmt.Errorf("unsupported instantiation type for function parameter: %v", fType.InstantiationType)
			}
		}

		inputValues := make([]reflect.Value, len(inputs))
		for i, v := range inputs {
			if i >= len(inputs) {
				return reflect.Value{}, fmt.Errorf("index out of range: %d", i)
			}
			inputValues[i] = v
		}

		results = fMeta.FuncValue.Call(inputValues)
	}

	if len(results) != len(fMeta.OutputTypes) {
		return reflect.Value{}, fmt.Errorf("function return value length mismatch: given %d, defined %d", len(results), len(fMeta.OutputTypes))
	}

	if fMeta.OutputTypes[len(fMeta.OutputTypes)-1] == errType.ID {
		if results[len(results)-1].Interface() != nil {
			return reflect.Value{}, results[len(results)-1].Interface().(error)
		}
	}

	// TODO: validate first result type
	return results[0], nil
}

func (t *TypeMeta) Instantiate(ctx context.Context, config *string, configs []Config, slots []Slot) (reflect.Value, error) {
	switch t.BasicType {
	case BasicTypeInteger, BasicTypeString, BasicTypeBool, BasicTypeNumber:
		return reflect.ValueOf(t.InstantiateByLiteral()), nil
	case BasicTypeStruct, BasicTypeArray, BasicTypeMap:
		switch t.InstantiationType {
		case InstantiationTypeUnmarshal:
			if config == nil {
				return reflect.Value{}, fmt.Errorf("config is nil when instantiate by unmarshal: %v", t.ID)
			}
			return t.InstantiateByUnmarshal(ctx, *config, slots)
		case InstantiationTypeFunction:
			return t.InstantiateByFunction(ctx, config, configs, slots)
		default:
			return reflect.Value{}, fmt.Errorf("unsupported instantiation type %v, for basicType: %v, typeID: %s",
				t.InstantiationType, t.BasicType, t.ID)
		}
	default:
		return reflect.Value{}, fmt.Errorf("unsupported basic type: %v", t.BasicType)
	}
}

func (t *TypeMeta) InstantiateByLiteral() any {
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
	default:
		panic(fmt.Sprintf("unsupported basic type: %s", t.BasicType))
	}
}

// When setting the final value, add this helper function:
func setValue(field reflect.Value, value reflect.Value) error {
	if field.Kind() == reflect.Ptr {
		// If field is a pointer but value isn't
		if value.Kind() != reflect.Ptr {
			// Create a new pointer of the appropriate type
			ptr := reflect.New(field.Type().Elem())
			// Set the pointed-to value
			ptr.Elem().Set(value)
			field.Set(ptr)
			return nil
		}
	}
	// Direct assignment for non-pointer fields
	field.Set(value)
	inter := field.Interface()
	_ = inter
	return nil
}
