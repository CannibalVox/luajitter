package luajitter

/*
#cgo CFLAGS: -I"C:/Users/Stephen Baynham/source/repos/LuaJIT/include/luajit-5.1"
#cgo LDFLAGS: -L"C:/Users/Stephen Baynham/source/repos/LuaJIT/lib/windows_amd64/luajit-5.1" -llua51
#include "go_luajit.h"
*/
import "C"
import (
	"errors"
	"github.com/baohavan/go-pointer"
	"unsafe"
)

//export releaseCGOHandle
func releaseCGOHandle(handle unsafe.Pointer) *C.lua_err {
	pointer.Unref(handle)
	return nil
}

//export callbackGoFunction
func callbackGoFunction(_L *C.lua_State, handle unsafe.Pointer, args C.lua_args) *C.lua_return {
	retVal := (*C.struct_lua_return)(C.chmalloc(luaReturnSize))
	handlePtr := pointer.Restore(handle)
	goFunction, ok := handlePtr.(LuaCallback)
	if !ok {
		retVal.err = GoErrorToLua(errors.New("attempted to call go function with non-callback object"))
		return retVal
	}

	goArgs := []interface{}{}
	state := vmMap[_L]
	argCount := int(args.valueCount)
	argsList := (*[1 << 30]*C.struct_lua_value)(unsafe.Pointer(args.values))

	for i := 0; i < argCount; i++ {
		singleArg := argsList[i]
		goArgs = append(goArgs, buildGoValue(state, singleArg))
	}

	retVals, err := goFunction(goArgs)
	retVal.valueCount = C.int(len(retVals))
	retVal.err = GoErrorToLua(err)

	valueArray := C.chmalloc(C.size_t(unsafe.Sizeof(uintptr(0))) * C.size_t(len(retVals)))
	allValues := (*[1 << 30]*C.struct_lua_value)(valueArray)
	for idx, singleVal := range retVals {
		var value *C.lua_value
		value, _, err = fromGoValue(state, singleVal)
		allValues[idx] = value
		if err != nil {
			break
		}
	}

	if err != nil {
		for _, value := range allValues {
			C.free_lua_value(_L, value)
		}
		retVal.valueCount = 0
		retVal.err = GoErrorToLua(err)
	} else {
		retVal.values = (**C.struct_lua_value)(valueArray)
	}

	return retVal
}
