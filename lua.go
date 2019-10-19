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

func LuaErrorToGo(err *C.lua_err) error {
	if err == nil {
		return nil
	}
	if err == C.INVALID_ERROR {
		panic("INVALID ERROR RAISED FROM LUA")
	}
	outErr := errors.New(C.GoString(err.message))
	return outErr
}

type LuaState struct {
	_l *C.lua_State
}

func NewState() *LuaState {
	vm := C.luaL_newstate()
	C.luaL_openlibs(vm)
	return &LuaState{
		_l: vm,
	}
}

func (s *LuaState) Close() error {
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
		result = buildGoValue(s, cResult.value)
	}

	return result, err
}

func (s *LuaState) GetGlobal(path string) (interface{}, error) {
	return s.getGlobal(path, false)
}

func (s *LuaState) setGlobal(path string, value interface{}, createIntermediateTables bool) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cValue, shouldFree, err := fromGoValue(s, value)
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
