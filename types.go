package luajitter

/*
#cgo CFLAGS: -I"C:/Users/Stephen Baynham/source/repos/LuaJIT/include/luajit-5.1"
#cgo LDFLAGS: -L"C:/Users/Stephen Baynham/source/repos/LuaJIT/lib/windows_amd64/luajit-5.1" -llua51
#include "go_luajit.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

var luaValueSize C.ulonglong = C.ulonglong(unsafe.Sizeof(C.lua_value{}))

func createLuaValue() *C.struct_lua_value {
	return (*C.struct_lua_value)(C.malloc(luaValueSize))
}

func fromGoValue(vm *LuaState, value interface{}) (cValue *C.struct_lua_value, shouldFree bool, err error) {
	if value == nil {
		return nil, false, nil
	}

	var outValue *C.struct_lua_value
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
			castV = float64(innerV)
		case float32:
			castV = float64(innerV)
		}
		outValue = createLuaValue()
		outValue.valueType = C.LUA_TNUMBER
		valData := (*C.double)(unsafe.Pointer(&outValue.data))
		*valData = C.double(castV)
		shouldFree = true

		break
	case bool:
		outValue = createLuaValue()
		outValue.valueType = C.LUA_TBOOLEAN
		valData := (*C._Bool)(unsafe.Pointer(&outValue.data))
		*valData = C._Bool(v)
		shouldFree = true

		break
	case string:
		outValue = createLuaValue()
		outValue.valueType = C.LUA_TSTRING
		valData := (**C.char)(unsafe.Pointer(&outValue.data))
		*valData = C.CString(v)
		valDataArg := (*C.size_t)(unsafe.Pointer(&outValue.dataArg))
		*valDataArg = C.size_t(len(v))
		shouldFree = true

		break
	case *LocalLuaFunction, *LocalLuaData:
		castV := v.(*LocalLuaData)
		if vm != castV.HomeVM() {
			return nil, false, errors.New("Attempt to use local data in wrong VM")
		}
		outValue = castV.LuaValue()
	}

	return outValue, shouldFree, nil
}

func buildGoValue(vm *LuaState, value *C.struct_lua_value) interface{} {
	if value == nil {
		return nil
	}

	switch value.valueType {
	case C.LUA_TNUMBER:
		union := (*C.double)(unsafe.Pointer(&value.data))
		retVal := float64(*union)
		C.free_lua_value(vm._l, value)
		return retVal
	case C.LUA_TBOOLEAN:
		union := (*C._Bool)(unsafe.Pointer(&value.data))
		retVal := bool(*union == (C._Bool)(true))
		C.free_lua_value(vm._l, value)
		return retVal
	case C.LUA_TSTRING:
		union := (**C.char)(unsafe.Pointer(&(value.data)))
		retVal := C.GoString(*union)
		C.free_lua_value(vm._l, value)
		return retVal
	case C.LUA_TFUNCTION:
		isCFunction := (*C._Bool)(unsafe.Pointer(&value.dataArg))
		if *isCFunction == (C._Bool)(false) {
			return &LocalLuaFunction{
				LocalLuaData{
					value:  value,
					homeVM: vm,
				},
			}
		}

		return &LocalLuaData{
			value:  value,
			homeVM: vm,
		}
	default:
		return &LocalLuaData{
			value:  value,
			homeVM: vm,
		}
	}

	return nil
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

type LocalLuaFunction struct {
	LocalLuaData
}

func (f *LocalLuaFunction) Call(args ...interface{}) ([]interface{}, error) {
	luaArgs := C.lua_args{
		valueCount: C.int(len(args)),
		values:     nil,
	}

	argsIn := []*C.struct_lua_value{}
	var err error
	for _, arg := range args {
		val, shouldFree, err := fromGoValue(f.HomeVM(), arg)
		if shouldFree {
			defer C.free_lua_value(f.HomeVM()._l, val)
		}
		if err != nil {
			break
		}

		argsIn = append(argsIn, val)
	}

	if len(argsIn) > 0 {
		luaArgs.values = &argsIn[0]
	}

	allRetVals := []interface{}{}
	if err == nil {
		retVal := C.call_function(f.HomeVM()._l, f.LuaValue(), luaArgs)
		if retVal.err != nil {
			defer C.free_lua_error(retVal.err)
			err = LuaErrorToGo(retVal.err)
		} else if retVal.valueCount > 0 {
			valueList := (*[1 << 30]*C.struct_lua_value)(unsafe.Pointer(retVal.values))
			for i := 0; i < int(retVal.valueCount); i++ {
				value := valueList[i]
				allRetVals = append(allRetVals, buildGoValue(f.HomeVM(), value))
			}
		}
	}

	return allRetVals, err
}
