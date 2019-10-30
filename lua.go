package luajitter

/*
#cgo CFLAGS: -I"C:/Users/Stephen Baynham/source/repos/LuaJIT/include/luajit-5.1"
#cgo LDFLAGS: -L"C:/Users/Stephen Baynham/source/repos/LuaJIT/lib/windows_amd64/luajit-5.1" -llua51
#include "go_luajit.h"
*/
import "C"
import (
	"unsafe"
)

var vmMap = make(map[*C.lua_State]*LuaState)

type LuaState struct {
	_l *C.lua_State
}

func NewState() *LuaState {
	vm := C.new_luajit_state()
	state := &LuaState{
		_l: vm,
	}
	vmMap[vm] = state
	return state
}

func (s *LuaState) Close() error {
	delete(vmMap, s._l)
	C.lua_close(s._l)
	return nil
}

func (s *LuaState) DoString(doString string) error {
	script := C.CString(doString)
	defer C.free(unsafe.Pointer(script))

	cErr := C.internal_dostring(s._l, script)

	defer C.free_lua_error(cErr)
	return LuaErrorToGo(cErr)
}

func (s *LuaState) getGlobal(path string, createIntermediateTables bool) (interface{}, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cResult := C.get_global(s._l, cPath, (C._Bool)(createIntermediateTables))
	defer C.free_lua_error(cResult.err)

	err := LuaErrorToGo(cResult.err)
	var result interface{}
	if cResult.value != nil {
		valArray := (*[1 << 30]*C.struct_lua_value)(unsafe.Pointer(&cResult.value))
		vals := buildGoValues(s, 1, valArray)
		if len(vals) > 0 {
			result = vals[0]
			if valArray[0] != nil {
				C.free_lua_value(s._l, valArray[0])
			}
		}
	}

	return result, err
}

func (s *LuaState) GetGlobal(path string) (interface{}, error) {
	return s.getGlobal(path, false)
}

func (s *LuaState) setGlobal(path string, value interface{}, createIntermediateTables bool) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cValue, shouldFree, err := fromGoValue(s, value, nil)
	if shouldFree {
		defer C.free_lua_value(s._l, cValue)
	}
	if err != nil {
		return err
	}

	cErr := C.set_global(s._l, cPath, cValue, (C._Bool)(createIntermediateTables))
	defer C.free_lua_error(cErr)

	return LuaErrorToGo(cErr)
}

func (s *LuaState) SetGlobal(path string, value interface{}) error {
	return s.setGlobal(path, value, false)
}

func (s *LuaState) InitGlobal(path string, value interface{}) error {
	return s.setGlobal(path, value, true)
}
