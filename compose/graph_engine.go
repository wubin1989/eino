package compose

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

func NewGraphFromDSL(ctx context.Context, dsl *GraphDSL) (*Graph[any, any], error) {
	newGraphOptions, err := genNewGraphOptions(dsl.StateType)
	if err != nil {
		return nil, err
	}

	g := NewGraph[any, any](newGraphOptions...)
	for i := range dsl.Nodes {
		node := dsl.Nodes[i]
		if err := graphAddNode(ctx, g, node); err != nil {
			return nil, err
		}
	}

	for _, edge := range dsl.Edges {
		if err := g.AddEdge(edge.From, edge.To); err != nil {
			return nil, err
		}
	}

	for i := range dsl.Branches {
		branch := dsl.Branches[i]
		if err := graphAddBranch(ctx, g, branch); err != nil {
			return nil, err
		}
	}

	return g, nil
}

type DSLRunner struct {
	Runnable[any, any]
	InputType *TypeMeta
}

func CompileGraph(ctx context.Context, dsl *GraphDSL) (*DSLRunner, error) {
	g, err := NewGraphFromDSL(ctx, dsl)
	if err != nil {
		return nil, err
	}

	compileOptions := genGraphCompileOptions(dsl)

	r, err := g.Compile(ctx, compileOptions...)
	if err != nil {
		return nil, err
	}

	inputType, ok := typeMap[dsl.InputType]
	if !ok {
		return nil, fmt.Errorf("type not found: %v", dsl.InputType)
	}

	return &DSLRunner{
		Runnable:  r,
		InputType: inputType,
	}, nil
}

func (d *DSLRunner) Invoke(ctx context.Context, in string, opts ...Option) (any, error) {
	v, err := d.InputType.Instantiate(ctx, []Config{{Value: in}})
	if err != nil {
		return nil, err
	}

	return d.Runnable.Invoke(ctx, v.Interface(), opts...)
}

func (d *DSLRunner) Stream(ctx context.Context, in string, opts ...Option) (*schema.StreamReader[any], error) {
	v, err := d.InputType.Instantiate(ctx, []Config{{Value: in}})
	if err != nil {
		return nil, err
	}
	return d.Runnable.Stream(ctx, v.Interface(), opts...)
}

func (d *DSLRunner) Collect(ctx context.Context, in string, opts ...Option) (any, error) {
	v, err := d.InputType.Instantiate(ctx, []Config{{Value: in}})
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[any](1)
	sw.Send(v.Interface(), nil)
	sw.Close()
	return d.Runnable.Collect(ctx, sr, opts...)
}

func (d *DSLRunner) Transform(ctx context.Context, in string, opts ...Option) (*schema.StreamReader[any], error) {
	v, err := d.InputType.Instantiate(ctx, []Config{{Value: in}})
	if err != nil {
		return nil, err
	}

	sr, sw := schema.Pipe[any](1)
	sw.Send(v.Interface(), nil)
	sw.Close()
	return d.Runnable.Transform(ctx, sr, opts...)
}

func graphAddNode(ctx context.Context, g *Graph[any, any], dsl *NodeDSL) error {
	addNodeOpts, err := genAddNodeOptions(dsl)
	if err != nil {
		return err
	}

	if dsl.GraphDSL != nil {
		subGraph, err := NewGraphFromDSL(ctx, dsl.GraphDSL)
		if err != nil {
			return err
		}

		compileOptions := genGraphCompileOptions(dsl.GraphDSL)

		addNodeOpts = append(addNodeOpts, WithGraphCompileOptions(compileOptions...))

		return g.AddGraphNode(dsl.Key, subGraph, addNodeOpts...)
	}

	if dsl.WorkflowDSL != nil {
		subGraph, err := NewWorkflowFromDSL(ctx, dsl.WorkflowDSL)
		if err != nil {
			return err
		}

		compileOptions := genWorkflowCompileOptions(dsl.WorkflowDSL)
		addNodeOpts = append(addNodeOpts, WithGraphCompileOptions(compileOptions...))

		return g.AddGraphNode(dsl.Key, subGraph, addNodeOpts...)
	}

	implMeta, ok := implMap[dsl.ImplID]
	if !ok {
		return fmt.Errorf("impl not found: %v", dsl.ImplID)
	}

	if implMeta.ComponentType == ComponentOfPassthrough {
		return g.AddPassthroughNode(dsl.Key, addNodeOpts...)
	}

	if implMeta.ComponentType == ComponentOfLambda {
		lambda := implMeta.Lambda
		if lambda == nil {
			return fmt.Errorf("lambda not found: %v", dsl.ImplID)
		}

		return g.AddLambdaNode(dsl.Key, lambda(), addNodeOpts...)
	}

	typeMeta, ok := typeMap[implMeta.TypeID]
	if !ok {
		return fmt.Errorf("type not found: %v", implMeta.TypeID)
	}

	result, err := typeMeta.Instantiate(ctx, dsl.Configs)
	if err != nil {
		return err
	}

	addFn, ok := comp2AddFn[implMeta.ComponentType]
	if !ok {
		return fmt.Errorf("add node function not found: %v", implMeta.ComponentType)
	}
	results := addFn.CallSlice(append([]reflect.Value{reflect.ValueOf(g.graph), reflect.ValueOf(dsl.Key), result, reflect.ValueOf(addNodeOpts)}))
	if len(results) != 1 {
		return fmt.Errorf("add node function return value length mismatch: given %d, defined %d", len(results), 1)
	}
	if results[0].Interface() != nil {
		return results[0].Interface().(error)
	}

	return nil
}

func graphAddBranch(_ context.Context, g *Graph[any, any], dsl *GraphBranchDSL) error {
	branch, _, err := genBranch(dsl.BranchDSL)
	if err != nil {
		return err
	}

	return g.AddBranch(dsl.FromNode, branch)
}

func genBranch(dsl *BranchDSL) (*GraphBranch, reflect.Type, error) {
	if len(dsl.EndNodes) <= 1 {
		return nil, nil, fmt.Errorf("graph branch must have more than one end nodes, actual= %v", dsl.EndNodes)
	}

	endNodes := make(map[string]bool, len(dsl.EndNodes))
	for _, node := range dsl.EndNodes {
		endNodes[node] = true
	}

	branchFunc, ok := branchFunctionMap[dsl.Condition]
	if !ok {
		return nil, nil, fmt.Errorf("branch function not found: %v", dsl.Condition)
	}

	// convert condition to GraphBranchCondition[any] or GraphStreamBranchCondition[any]
	// create the *GraphBranch
	var (
		branch    *GraphBranch
		inputType reflect.Type
	)

	if !branchFunc.IsStream {
		inputType = branchFunc.FuncValue.Type().In(1)

		condition := func(ctx context.Context, in any) (string, error) {
			if !reflect.TypeOf(in).AssignableTo(branchFunc.InputType) {
				return "", fmt.Errorf("branch condition expects input type of %v, actual= %v", branchFunc.InputType, reflect.TypeOf(in))
			}

			results := branchFunc.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(in)})
			if len(results) != 2 {
				return "", fmt.Errorf("branch condition function return value length mismatch: given %d, defined %d", len(results), 2)
			}

			if results[1].Interface() != nil {
				return "", results[1].Interface().(error)
			}

			return results[0].Interface().(string), nil
		}
		branch = NewGraphBranch(condition, endNodes)
	} else {
		inputType = branchFunc.StreamConverter.inputType()

		condition := func(ctx context.Context, in *schema.StreamReader[any]) (string, error) {
			converted, err := branchFunc.StreamConverter.convertFromAny(in)
			if err != nil {
				return "", err
			}

			results := branchFunc.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), converted})

			if results[1].Interface() != nil {
				return "", results[1].Interface().(error)
			}

			return results[0].Interface().(string), nil
		}
		branch = NewStreamGraphBranch(condition, endNodes)
	}

	return branch, inputType, nil
}

func assignSlot(destValue reflect.Value, taken any, toPaths FieldPath) (reflect.Value, error) {
	if len(toPaths) == 0 {
		return reflect.Value{}, fmt.Errorf("empty fieldPath")
	}

	var (
		originalDestValue = destValue
		parentMap         reflect.Value
		parentKey         string
	)

	for {
		path := toPaths[0]
		toPaths = toPaths[1:]
		if len(toPaths) == 0 {
			toSet := reflect.ValueOf(taken)

			if destValue.Kind() == reflect.Map {
				key, err := checkAndExtractToMapKey(path, destValue, toSet)
				if err != nil {
					return reflect.Value{}, err
				}

				destValue.SetMapIndex(key, toSet)

				if parentMap.IsValid() {
					parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
				}

				return originalDestValue, nil
			}

			if destValue.Kind() == reflect.Slice {
				instantiateIfNeeded(destValue)

				elem, err := checkAndExtractToSliceIndex(path, destValue)
				if err != nil {
					return reflect.Value{}, err
				}

				if !toSet.Type().AssignableTo(elem.Type()) {
					return reflect.Value{}, fmt.Errorf("field mapping to a slice element has a mismatched type. from=%v, to=%v", toSet.Type(), destValue.Type().Elem())
				}

				elem.Set(toSet)

				if parentMap.IsValid() {
					parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
				}

				return originalDestValue, nil
			}

			field, err := checkAndExtractToField(path, destValue, toSet)
			if err != nil {
				return reflect.Value{}, err
			}

			field.Set(toSet)

			if parentMap.IsValid() {
				parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
			}

			return originalDestValue, nil
		}

		if destValue.Kind() == reflect.Map {
			if !reflect.TypeOf(path).AssignableTo(destValue.Type().Key()) {
				return reflect.Value{}, fmt.Errorf("field mapping to a map key but output is not a map with string key, type=%v", destValue.Type())
			}

			keyValue := reflect.ValueOf(path)
			valueValue := destValue.MapIndex(keyValue)
			if !valueValue.IsValid() {
				valueValue = newInstanceByType(destValue.Type().Elem())
				destValue.SetMapIndex(keyValue, valueValue)
			}

			if parentMap.IsValid() {
				parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
			}

			parentMap = destValue
			parentKey = path
			destValue = valueValue

			continue
		}

		if destValue.Kind() == reflect.Slice {
			elem, err := checkAndExtractToSliceIndex(path, destValue)
			if err != nil {
				return reflect.Value{}, err
			}

			instantiateIfNeeded(elem)

			if parentMap.IsValid() {
				parentMap.SetMapIndex(reflect.ValueOf(parentKey), destValue)
			}

			parentMap = reflect.Value{}
			parentKey = ""
			destValue = elem

			continue
		}

		ptrValue := destValue
		for destValue.Kind() == reflect.Ptr {
			destValue = destValue.Elem()
		}

		if destValue.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("field mapping to a struct field but output is not a struct, type=%v", destValue.Type())
		}

		field := destValue.FieldByName(path)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("field mapping to a struct field, but field not found. field=%v, outputType=%v", path, destValue.Type())
		}

		if !field.CanSet() {
			return reflect.Value{}, fmt.Errorf("field mapping to a struct field, but field not exported. field=%v, outputType=%v", path, destValue.Type())
		}

		instantiateIfNeeded(field)

		if parentMap.IsValid() {
			parentMap.SetMapIndex(reflect.ValueOf(parentKey), ptrValue)
		}

		parentMap = reflect.Value{}
		parentKey = ""

		destValue = field
	}
}

func (t *TypeMeta) InstantiateByUnmarshal(ctx context.Context, conf Config) (reflect.Value, error) {
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

	value := conf.Value
	if value == nil {
		value = "{}"
	}

	strValue, ok := value.(string)
	if !ok {
		return reflect.Value{}, fmt.Errorf("config value is not a string when instantiate with unmarshal: %v", value)
	}
	err := sonic.Unmarshal([]byte(strValue), instance)
	if err != nil {
		return reflect.Value{}, err
	}

	rValue := reflect.ValueOf(instance)
	if !isPtr {
		rValue = rValue.Elem()
	}

	for _, slot := range conf.Slots {
		slotType, ok := typeMap[slot.TypeID]
		if !ok {
			return reflect.Value{}, fmt.Errorf("type not found: %v", slot.TypeID)
		}

		slotInstance, err := slotType.Instantiate(ctx, slot.Configs)
		if err != nil {
			return reflect.Value{}, err
		}

		rValue, err = assignSlot(rValue, slotInstance.Interface(), slot.Path)
		if err != nil {
			return reflect.Value{}, err
		}
	}

	return rValue, nil
}

func (t *TypeMeta) InstantiateByFunction(ctx context.Context, configs []Config) (reflect.Value, error) {
	fMeta := t.FunctionMeta
	if fMeta == nil {
		return reflect.Value{}, fmt.Errorf("function meta not found: %v", t.ID)
	}

	var results []reflect.Value

	if len(fMeta.InputTypes) == 0 {
		results = fMeta.FuncValue.Call([]reflect.Value{})
	} else {
		inputs := make([]reflect.Value, 0, len(configs)+1)

		// add a context parameter at the front if needed
		first := fMeta.InputTypes[0]
		if first == ctxType.ID {
			configs = append([]Config{
				{
					isCtx: true,
				},
			}, configs...)
		}

		for i, conf := range configs {
			if conf.isCtx {
				inputs = append(inputs, reflect.ValueOf(ctx))
				continue
			}

			if conf.Slot != nil {
				slot := conf.Slot
				slotType, ok := typeMap[slot.TypeID]
				if !ok {
					return reflect.Value{}, fmt.Errorf("type not found: %v", slot.TypeID)
				}

				slotInstance, err := slotType.Instantiate(ctx, slot.Configs)
				if err != nil {
					return reflect.Value{}, err
				}

				if len(slot.Path) > 0 {
					return reflect.Value{}, fmt.Errorf("non empty slot path for slice element. path= %s", slot.Path)
				}

				inputs = append(inputs, slotInstance)

				continue
			}

			var fTypeID TypeID
			if i >= len(fMeta.InputTypes) {
				if !fMeta.IsVariadic {
					return reflect.Value{}, fmt.Errorf("configs length mismatch: given %d, defined %d", len(configs), len(fMeta.InputTypes))
				}
				fTypeID = fMeta.InputTypes[len(fMeta.InputTypes)-1]
			} else {
				fTypeID = fMeta.InputTypes[i]
			}

			fType, ok := typeMap[fTypeID]
			if !ok {
				return reflect.Value{}, fmt.Errorf("type not found: %v", fTypeID)
			}

			oneValue, err := fType.Instantiate(ctx, []Config{conf})
			if err != nil {
				return reflect.Value{}, err
			}

			inputs = append(inputs, oneValue)
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

func (t *TypeMeta) Instantiate(ctx context.Context, configs []Config) (reflect.Value, error) {
	if len(configs) == 0 {
		return reflect.Value{}, fmt.Errorf("no configs provided")
	}

	switch t.BasicType {
	case BasicTypeInteger, BasicTypeString, BasicTypeBool, BasicTypeNumber:
		return t.InstantiateByLiteral(configs[0].Value)
	case BasicTypeStruct, BasicTypeArray, BasicTypeMap:
		switch t.InstantiationType {
		case InstantiationTypeUnmarshal:
			if len(configs) > 1 {
				return reflect.Value{}, fmt.Errorf("configs are not allowed when instantiate by unmarshal: %v", t.ID)
			}

			return t.InstantiateByUnmarshal(ctx, configs[0])
		case InstantiationTypeFunction:
			return t.InstantiateByFunction(ctx, configs)
		default:
			return reflect.Value{}, fmt.Errorf("unsupported instantiation type %v, for basicType: %v, typeID: %s",
				t.InstantiationType, t.BasicType, t.ID)
		}
	default:
		return reflect.Value{}, fmt.Errorf("unsupported basic type: %v", t.BasicType)
	}
}

func (t *TypeMeta) InstantiateByLiteral(value any) (reflect.Value, error) {
	switch t.BasicType {
	case BasicTypeString:
		s, ok := value.(string)
		if !ok {
			return reflect.Value{}, fmt.Errorf("value is not a string: %v", value)
		}
		if t.IsPtr {
			if t.ID == TypeIDStringPtr {
				return reflect.ValueOf(generic.PtrOf(s)), nil
			} else {
				if t.ReflectType == nil {
					return reflect.Value{}, fmt.Errorf("reflect type is nil: %v", value)
				}
				return reflect.ValueOf(generic.PtrOf(s)).Convert(*t.ReflectType), nil
			}
		}

		if t.ID == TypeIDString {
			return reflect.ValueOf(s), nil
		}
		if t.ReflectType == nil {
			return reflect.Value{}, fmt.Errorf("reflect type is nil: %v", value)
		}
		return reflect.ValueOf(s).Convert(*t.ReflectType), nil
	case BasicTypeInteger:
		i, ok := value.(int)
		if !ok {
			return reflect.Value{}, fmt.Errorf("value is not an integer: %v", value)
		}

		if t.IntegerType == nil {
			return reflect.Value{}, fmt.Errorf("basic type is %v, integer type is nil", t.BasicType)
		}
		i64 := int64(i)
		var result reflect.Value
		var err error
		switch *t.IntegerType {
		case IntegerTypeInt8:
			result, err = handleIntegerType(i64, math.MinInt8, math.MaxInt8, int8(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDInt8, TypeIDInt8Ptr)
		case IntegerTypeInt16:
			result, err = handleIntegerType(i64, math.MinInt16, math.MaxInt16, int16(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDInt16, TypeIDInt16Ptr)
		case IntegerTypeInt32:
			result, err = handleIntegerType(i64, math.MinInt32, math.MaxInt32, int32(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDInt32, TypeIDInt32Ptr)
		case IntegerTypeInt64:
			result, err = handleIntegerType(i64, math.MinInt64, math.MaxInt64, i64, t.IsPtr, t.ReflectType, t.ID, TypeIDInt64, TypeIDInt64Ptr)
		case IntegerTypeUint8:
			result, err = handleIntegerType(i64, 0, math.MaxUint8, uint8(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint8, TypeIDUint8Ptr)
		case IntegerTypeUint16:
			result, err = handleIntegerType(i64, 0, math.MaxUint16, uint16(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint16, TypeIDUint16Ptr)
		case IntegerTypeUint32:
			result, err = handleIntegerType(i64, 0, math.MaxUint32, uint32(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint32, TypeIDUint32Ptr)
		case IntegerTypeUint64:
			result, err = handleIntegerType(i64, 0, math.MaxUint64, uint64(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint64, TypeIDUint64Ptr)
		case IntegerTypeInt:
			if strconv.IntSize == 32 {
				result, err = handleIntegerType(i64, math.MinInt32, math.MaxInt32, int(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDInt, TypeIDIntPtr)
			} else {
				result, err = handleIntegerType(i64, math.MinInt64, math.MaxInt64, int(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDInt, TypeIDIntPtr)
			}
		case IntegerTypeUInt:
			if strconv.IntSize == 32 {
				result, err = handleIntegerType(i64, 0, math.MaxUint32, uint(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint, TypeIDUintPtr)
			} else {
				if i64 < 0 {
					return reflect.Value{}, fmt.Errorf("overflow: value %d is out of range for uint (64-bit system)", i64)
				}
				result, err = handleIntegerType(i64, 0, math.MaxUint64, uint(i64), t.IsPtr, t.ReflectType, t.ID, TypeIDUint, TypeIDUintPtr)
			}
		default:
			return reflect.Value{}, fmt.Errorf("unsupported integer type: %v", *t.IntegerType)
		}
		if err != nil {
			return reflect.Value{}, err
		}
		return result, nil
	case BasicTypeNumber:
		f64, ok := value.(float64)
		if !ok {
			return reflect.Value{}, fmt.Errorf("value is not a float64: %v", value)
		}

		if t.FloatType == nil {
			return reflect.Value{}, fmt.Errorf("basic type is %v, float type is nil", t.BasicType)
		}

		var result reflect.Value
		var err error
		switch *t.FloatType {
		case FloatTypeFloat32:
			result, err = handleFloatType(f64, math.SmallestNonzeroFloat32, math.MaxFloat32, float32(f64), t.IsPtr, t.ReflectType, t.ID, TypeIDFloat32, TypeIDFloat32Ptr)
		case FloatTypeFloat64:
			result, err = handleFloatType(f64, math.SmallestNonzeroFloat64, math.MaxFloat64, f64, t.IsPtr, t.ReflectType, t.ID, TypeIDFloat64, TypeIDFloat64Ptr)
		default:
			return reflect.Value{}, fmt.Errorf("unsupported float type: %v", *t.FloatType)
		}
		if err != nil {
			return reflect.Value{}, err
		}
		return result, nil
	case BasicTypeBool:
		b, ok := value.(bool)
		if !ok {
			return reflect.Value{}, fmt.Errorf("value is not a bool: %v", value)
		}
		if t.IsPtr {
			if t.ID == TypeIDBoolPtr {
				return reflect.ValueOf(generic.PtrOf(b)), nil
			}

			if t.ReflectType == nil {
				return reflect.Value{}, fmt.Errorf("reflect type is nil: %v", value)
			}
			return reflect.ValueOf(generic.PtrOf(b)).Convert(*t.ReflectType), nil
		}

		if t.ID == TypeIDBool {
			return reflect.ValueOf(b), nil
		}
		if t.ReflectType == nil {
			return reflect.Value{}, fmt.Errorf("reflect type is nil: %v", value)
		}
		return reflect.ValueOf(b).Convert(*t.ReflectType), nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported basic type: %s", t.BasicType)
	}
}

func handleIntegerType(value int64, min int64, max uint64, convertedValue any, isPtr bool, reflectType *reflect.Type, currentTypeID, nonPtrTypeID, ptrTypeID TypeID) (reflect.Value, error) {
	if value < min || uint64(value) > max {
		return reflect.Value{}, fmt.Errorf("overflow: value %d is out of range", value)
	}
	if isPtr {
		if currentTypeID == ptrTypeID {
			return reflect.ValueOf(generic.PtrOf(convertedValue)), nil
		}
		if reflectType == nil {
			return reflect.Value{}, fmt.Errorf("reflect type is nil")
		}
		return reflect.ValueOf(generic.PtrOf(convertedValue)).Convert(*reflectType), nil
	}
	if currentTypeID == nonPtrTypeID {
		return reflect.ValueOf(convertedValue), nil
	}
	if reflectType == nil {
		return reflect.Value{}, fmt.Errorf("reflect type is nil")
	}
	return reflect.ValueOf(convertedValue).Convert(*reflectType), nil
}

func handleFloatType(value float64, min, max float64, convertedValue any, isPtr bool, reflectType *reflect.Type, currentTypeID, nonPtrTypeID, ptrTypeID TypeID) (reflect.Value, error) {
	if value < min || value > max {
		return reflect.Value{}, fmt.Errorf("overflow: value %f is out of range", value)
	}
	if isPtr {
		if currentTypeID == ptrTypeID {
			return reflect.ValueOf(generic.PtrOf(convertedValue)), nil
		}
		if reflectType == nil {
			return reflect.Value{}, fmt.Errorf("reflect type is nil")
		}
		return reflect.ValueOf(generic.PtrOf(convertedValue)).Convert(*reflectType), nil
	}
	if currentTypeID == nonPtrTypeID {
		return reflect.ValueOf(convertedValue), nil
	}
	if reflectType == nil {
		return reflect.Value{}, fmt.Errorf("reflect type is nil")
	}
	return reflect.ValueOf(convertedValue).Convert(*reflectType), nil
}

// checkAndExtractToSliceIndex extracts and validates the slice index from the path
func checkAndExtractToSliceIndex(path string, destValue reflect.Value) (reflect.Value, error) {
	index, err := extractSliceIndexFromPath(path)
	if err != nil {
		return reflect.Value{}, err
	}

	if index >= destValue.Len() {
		// Reslice destValue to contain enough elements
		newLen := index + 1
		capacity := destValue.Cap()
		if newLen > capacity {
			// If the new length exceeds the capacity, create a new slice
			newSlice := reflect.MakeSlice(destValue.Type(), newLen, newLen)
			reflect.Copy(newSlice, destValue)
			destValue.Set(newSlice)
		} else {
			// If the new length is within the capacity, just reslice
			destValue.SetLen(newLen)
		}
	}

	return destValue.Index(index), nil
}

func extractSliceIndexFromPath(path string) (int, error) {
	// Extract the index part in the form of "[1]"
	startIndex := strings.Index(path, "[")
	endIndex := strings.Index(path, "]")
	if startIndex == -1 || endIndex == -1 || endIndex <= startIndex+1 {
		return -1, fmt.Errorf("invalid slice index syntax in path: %s", path)
	}
	indexStr := path[startIndex+1 : endIndex]

	// Parse the index from the extracted string
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return -1, fmt.Errorf("invalid slice index in path: %s", path)
	}

	// Check if the index is within the bounds of the slice
	if index < 0 {
		return index, fmt.Errorf("slice index out of bounds: %d", index)
	}

	return index, nil
}

func stateHandler(handler *StateHandler) func(ctx context.Context, in any, state any) (any, error) {
	return func(ctx context.Context, in any, state any) (any, error) {
		if !reflect.TypeOf(in).AssignableTo(handler.InputType) {
			return "", fmt.Errorf("state handler expects input type of %v, actual= %v", handler.InputType, reflect.TypeOf(in))
		}

		if !reflect.TypeOf(state).AssignableTo(handler.StateType) {
			return "", fmt.Errorf("state handler expects state type of %v, actual= %v", handler.StateType, reflect.TypeOf(state))
		}

		results := handler.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(in), reflect.ValueOf(state)})
		if len(results) != 2 {
			return "", fmt.Errorf("state handler return value length mismatch: given %d, defined %d", len(results), 2)
		}

		if results[1].Interface() != nil {
			return "", results[1].Interface().(error)
		}

		return results[0].Interface(), nil
	}
}

func streamStateHandler(handler *StateHandler) func(ctx context.Context, in *schema.StreamReader[any], state any) (*schema.StreamReader[any], error) {
	return func(ctx context.Context, in *schema.StreamReader[any], state any) (*schema.StreamReader[any], error) {
		converted, err := handler.StreamConverter.convertFromAny(in)
		if err != nil {
			return nil, err
		}

		results := handler.FuncValue.Call([]reflect.Value{reflect.ValueOf(ctx), converted, reflect.ValueOf(state)})

		if results[1].Interface() != nil {
			return nil, results[1].Interface().(error)
		}

		return handler.StreamConverter.packToAny(results[0]), nil
	}
}

func genNewGraphOptions(stateType *TypeID) ([]NewGraphOption, error) {
	newGraphOptions := make([]NewGraphOption, 0)
	if stateType != nil {
		stateType, ok := typeMap[*stateType]
		if !ok {
			return nil, fmt.Errorf("unknown state type: %v", *stateType)
		}

		if stateType.ReflectType == nil {
			return nil, fmt.Errorf("state type not found: %v", stateType.ID)
		}

		stateFn := func(ctx context.Context) any {
			return newInstanceByType(*stateType.ReflectType).Interface()
		}
		newGraphOptions = append(newGraphOptions, WithGenLocalState(stateFn))
	}

	return newGraphOptions, nil
}

func genAddNodeOptions(dsl *NodeDSL) ([]GraphAddNodeOpt, error) {
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

	if dsl.StatePreHandler != nil {
		handler, ok := stateHandlerMap[*dsl.StatePreHandler]
		if !ok {
			return nil, fmt.Errorf("unknown state pre handler: %v", *dsl.StatePreHandler)
		}

		if !handler.IsStream {
			handlerFunc := stateHandler(handler)
			addNodeOpts = append(addNodeOpts, WithStatePreHandler(handlerFunc))
		} else {
			handlerFunc := streamStateHandler(handler)
			addNodeOpts = append(addNodeOpts, WithStreamStatePreHandler(handlerFunc))
		}
	}

	if dsl.StatePostHandler != nil {
		handler, ok := stateHandlerMap[*dsl.StatePostHandler]
		if !ok {
			return nil, fmt.Errorf("unknown state post handler: %v", *dsl.StatePostHandler)
		}

		if !handler.IsStream {
			handlerFunc := stateHandler(handler)
			addNodeOpts = append(addNodeOpts, WithStatePostHandler(handlerFunc))
		} else {
			handlerFunc := streamStateHandler(handler)
			addNodeOpts = append(addNodeOpts, WithStreamStatePostHandler(handlerFunc))
		}
	}

	return addNodeOpts, nil
}

func genGraphCompileOptions(dsl *GraphDSL) []GraphCompileOption {
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
	return compileOptions
}
