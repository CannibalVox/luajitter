package luajitter

/*
#cgo !windows pkg-config: luajit
#cgo windows CFLAGS: -I${SRCDIR}/include
#cgo windows LDFLAGS: -L${SRCDIR} -llua51
#include "go_luajit.h"
*/
import "C"

import (
	"errors"
	"github.com/baohavan/go-pointer"
	"unsafe"
)

func outlyingAllocs() int {
	return int(C.outlying_allocs())
}

func clearAllocs() {
	C.clear_allocs()
}

var luaValueSize C.size_t = C.size_t(unsafe.Sizeof(C.lua_value{}))
var luaReturnSize C.size_t = C.size_t(unsafe.Sizeof(C.lua_return{}))

func createLuaValue() *C.struct_lua_value {
	return (*C.struct_lua_value)(C.chmalloc(luaValueSize))
}

func fromGoValue(vm *LuaState, value interface{}, outValue *C.struct_lua_value) (cValue *C.struct_lua_value, shouldFree bool, err error) {
	if value == nil {
		return nil, false, nil
	}

	shouldFree = false

	switch v := value.(type) {
	case uint64, uint32, int32, int64, int, uint, float64, float32:
		var castV float64
		switch innerV := v.(type) {
		case uint64:
			castV = float64(innerV)
		case uint32:
			castV = float64(innerV)
		case int32:
			castV = float64(innerV)
		case int64:
			castV = float64(innerV)
		case int:
			castV = float64(innerV)
		case uint:
			castV = float64(innerV)
		case float64:
			castV = innerV
		case float32:
			castV = float64(innerV)
		}
		if outValue == nil {
			outValue = createLuaValue()
		}
		outValue.valueType = C.LUA_TNUMBER
		valData := (*C.double)(unsafe.Pointer(&outValue.data))
		*valData = C.double(castV)
		shouldFree = true
	case bool:
		if outValue == nil {
			outValue = createLuaValue()
		}

		outValue.valueType = C.LUA_TBOOLEAN
		valData := (*C._Bool)(unsafe.Pointer(&outValue.data))
		*valData = C._Bool(v)
		shouldFree = true
	case string:
		if outValue == nil {
			outValue = createLuaValue()
		}

		outValue.valueType = C.LUA_TSTRING
		valData := (**C.char)(unsafe.Pointer(&outValue.data))
		*valData = C.CString(v)
		C.increment_allocs()
		valDataArg := (*C.size_t)(unsafe.Pointer(&outValue.dataArg))
		*valDataArg = C.size_t(len(v))
		shouldFree = true
	case *LocalLuaFunction, *LocalLuaData:
		castV := v.(*LocalLuaData)
		if outValue != nil {
			return outValue, false, errors.New("incorrectly-allocated ")
		}
		if vm != castV.HomeVM() {
			return nil, false, errors.New("attempt to use local data in wrong VM")
		}
		outValue = castV.LuaValue()
	case func([]interface{}) ([]interface{}, error):
		if outValue == nil {
			outValue = createLuaValue()
		}

		outValue.valueType = C.LUA_TUNLOADEDCALLBACK
		ptr := pointer.Save(v)

		valData := (*unsafe.Pointer)(unsafe.Pointer(&outValue.data))
		*valData = ptr
		shouldFree = true
	default:
		return nil, false, errors.New("cannot marshal unknown type into lua")
	}

	return outValue, shouldFree, nil
}

func buildGoValues(vm *LuaState, count int, values *[1 << 30]*C.struct_lua_value) []interface{} {
	goValues := make([]interface{}, count)
	for i := 0; i < count; i++ {
		value := values[i]
		if value == nil {
			goValues[i] = nil
			continue
		}

		switch value.valueType {
		case C.LUA_TNUMBER:
			union := (*C.double)(unsafe.Pointer(&value.data))
			goValues[i] = float64(*union)
			continue
		case C.LUA_TBOOLEAN:
			union := (*C._Bool)(unsafe.Pointer(&value.data))
			goValues[i] = bool(*union == (C._Bool)(true))
			continue
		case C.LUA_TSTRING:
			union := (**C.char)(unsafe.Pointer(&(value.data)))
			goValues[i] = C.GoString(*union)
			continue
		case C.LUA_TFUNCTION:
			isCFunction := (*C._Bool)(unsafe.Pointer(&value.dataArg))
			if *isCFunction == (C._Bool)(false) {
				//NULL out the index to stop it from being freed
				values[i] = nil
				goValues[i] = &LocalLuaFunction{
					LocalLuaData{
						value:  value,
						homeVM: vm,
					},
				}
				continue
			}

			fallthrough
		default:
			//NULL out the index to stop it from being freed
			values[i] = nil
			goValues[i] = &LocalLuaData{
				value:  value,
				homeVM: vm,
			}
		}
	}

	return goValues
}

type LocalLuaData struct {
	value  *C.struct_lua_value
	homeVM *LuaState
}

func (d *LocalLuaData) LuaValue() *C.struct_lua_value {
	return d.value
}

func (d *LocalLuaData) HomeVM() *LuaState {
	return d.homeVM
}

func (d *LocalLuaData) Close() error {
	if d.value != nil {
		C.free_lua_value(d.homeVM._l, d.value)
		d.value = nil
	}

	return nil
}

type LocalLuaFunction struct {
	LocalLuaData
}

func (f *LocalLuaFunction) Call(args ...interface{}) ([]interface{}, error) {
	luaArgs := C.lua_args{
		valueCount: C.int(len(args)),
		values:     nil,
	}

	argsIn := make([]*C.struct_lua_value, len(args))
	var err error
	for ind, arg := range args {
		val, shouldFree, err := fromGoValue(f.HomeVM(), arg, nil)
		if shouldFree {
			defer C.free_lua_value(f.HomeVM()._l, val)
		}
		if err != nil {
			break
		}

		argsIn[ind] = val
	}

	if len(argsIn) > 0 {
		luaArgs.values = &argsIn[0]
	}

	var allRetVals []interface{}
	if err == nil {
		retVal := C.call_function(f.HomeVM()._l, f.LuaValue(), luaArgs)
		if retVal.err != nil {
			defer C.free_lua_error(retVal.err)
			err = LuaErrorToGo(retVal.err)
		} else if retVal.valueCount > 0 {
			defer C.free_lua_return(f.HomeVM()._l, retVal, C._Bool(true))
			valueList := (*[1 << 30]*C.struct_lua_value)(unsafe.Pointer(retVal.values))
			allRetVals = buildGoValues(f.HomeVM(), int(retVal.valueCount), valueList)
		}
	}

	return allRetVals, err
}
