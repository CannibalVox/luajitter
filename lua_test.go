package luajitter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func closeVM(t *testing.T, vm *LuaState) {
	err := vm.Close()
	require.Nil(t, err)
}
func TestSimpleGlobal(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.SetGlobal("wow", 2)
	require.Nil(t, err)

	val, err := vm.GetGlobal("wow")
	require.Nil(t, err)
	require.NotNil(t, val)

	number, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, 2.0, number)

	require.Equal(t, 0, outlyingAllocs())
}

func TestInitGlobal(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.InitGlobal("test.test2.test3", "value")
	require.Nil(t, err)

	val, err := vm.GetGlobal("test.test2.test3")
	require.Nil(t, err)
	require.NotNil(t, val)
	strVal, ok := val.(string)
	require.True(t, ok)
	require.Equal(t, "value", strVal)

	tableObj, err := vm.GetGlobal("test.test2")
	require.Nil(t, err)
	require.NotNil(t, tableObj)
	table, ok := tableObj.(*LocalLuaData)
	require.True(t, ok)
	require.NotNil(t, table)
	require.Equal(t, 5, int(table.value.valueType))

	err = table.Close()
	require.Nil(t, err)

	require.Equal(t, 0, outlyingAllocs())
}

const fibo string = `
function fib(val)
	if val < 2 then 
		return val
	end

	return fib(val-2) + fib(val-1)
end

print(fib(5))
`

func TestDoStringAndCall(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.DoString(fibo)
	require.Nil(t, err)

	val, err := vm.GetGlobal("fib")
	require.Nil(t, err)
	require.NotNil(t, val)

	fibFunc, ok := val.(*LocalLuaFunction)
	require.True(t, ok)
	require.NotNil(t, fibFunc)

	out, err := fibFunc.Call(7)
	require.Nil(t, err)
	require.NotNil(t, out)
	require.Len(t, out, 1)
	require.NotNil(t, out[0])

	outNumber, ok := out[0].(float64)
	require.True(t, ok)
	require.Equal(t, 13.0, outNumber)

	err = fibFunc.Close()
	require.Nil(t, err)

	require.Equal(t, 0, outlyingAllocs())
}

const multiRet string = `
function multiCall()
	return 9,"testing",false
end
`

func TestMultiRetCall(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	require := require.New(t)

	err := vm.DoString(multiRet)
	require.Nil(err)

	val, err := vm.GetGlobal("multiCall")
	require.Nil(err)
	require.NotNil(val)

	multiCallFunc, ok := val.(*LocalLuaFunction)
	require.True(ok)
	require.NotNil(multiCallFunc)

	out, err := multiCallFunc.Call()
	require.Nil(err)
	require.NotNil(out)
	require.Len(out, 3)
	require.Equal(9.0, out[0])
	require.Equal("testing", out[1])
	require.Equal(false, out[2])

	err = multiCallFunc.Close()
	require.Nil(err)

	require.Equal(0, outlyingAllocs())
}

func TestDoStringAndCallNil(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.DoString("function retNil() return nil end")
	require.Nil(t, err)

	val, err := vm.GetGlobal("retNil")
	require.Nil(t, err)
	require.NotNil(t, val)

	fibFunc, ok := val.(*LocalLuaFunction)
	require.True(t, ok)
	require.NotNil(t, fibFunc)

	out, err := fibFunc.Call()
	require.Nil(t, err)
	require.NotNil(t, out)
	require.Len(t, out, 1)
	require.Nil(t, out[0])

	err = fibFunc.Close()
	require.Nil(t, err)

	require.Equal(t, 0, outlyingAllocs())
}

func TestDoStringWithError(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.DoString(`error("some error")`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "some error")

	require.Equal(t, 0, outlyingAllocs())
}

func TestDoCallWithError(t *testing.T) {
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.DoString(`function errcall(msg) error(msg) end`)
	require.Nil(t, err)

	val, err := vm.GetGlobal("errcall")
	require.Nil(t, err)
	require.NotNil(t, val)

	fibFunc, ok := val.(*LocalLuaFunction)
	require.True(t, ok)
	require.NotNil(t, fibFunc)

	out, err := fibFunc.Call("another error")
	require.NotNil(t, err)
	require.Len(t, out, 0)
	require.Contains(t, err.Error(), "another error")

	err = fibFunc.Close()
	require.Nil(t, err)

	require.Equal(t, 0, outlyingAllocs())
}

var callbackArgs []interface{}

func SomeErrorCallback(args []interface{}) ([]interface{}, error) {
	callbackArgs = args
	return []interface{}{
		"test",
		5,
		true,
		SomeErrorCallback,
	}, errors.New("WOW ERROR")
}
func TestDoErrorCallback(t *testing.T) {
	require := require.New(t)
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.InitGlobal("test.error_callback", LuaCallback(SomeErrorCallback))
	require.Nil(err)

	err = vm.DoString(`
function doErrorCallback()
	return test.error_callback(5, "bleh", nil, {})
end
`)
	require.Nil(err)

	errorFuncObj, err := vm.GetGlobal("doErrorCallback")
	require.Nil(err)
	require.NotNil(errorFuncObj)

	errorF, ok := errorFuncObj.(*LocalLuaFunction)
	require.True(ok)
	require.NotNil(errorF)

	retVals, err := errorF.Call()
	require.NotNil(retVals)
	require.Len(retVals, 0)

	require.NotNil(err)
	require.Equal("WOW ERROR", err.Error())

	require.Len(callbackArgs, 4)
	require.Equal(5.0, callbackArgs[0])
	require.Equal("bleh", callbackArgs[1])
	require.Nil(callbackArgs[2])
	require.IsType(&LocalLuaData{}, callbackArgs[3])

	require.Equal(0, outlyingAllocs())
}

func SomeCallback(args []interface{}) ([]interface{}, error) {
	callbackArgs = args
	return []interface{}{
		"test",
		5,
		true,
		SomeCallback,
	}, nil
}
func TestDoCallback(t *testing.T) {
	require := require.New(t)
	clearAllocs()
	vm := NewState()
	defer closeVM(t, vm)

	err := vm.InitGlobal("test.callback", LuaCallback(SomeCallback))
	require.Nil(err)

	err = vm.DoString(`
function doCallback()
	return test.callback(5, "bleh", nil, {})
end
`)
	require.Nil(err)

	funcObj, err := vm.GetGlobal("doCallback")
	require.Nil(err)
	require.NotNil(funcObj)

	f, ok := funcObj.(*LocalLuaFunction)
	require.True(ok)
	require.NotNil(f)

	retVals, err := f.Call()
	require.NotNil(retVals)
	require.Len(retVals, 4)
	require.Equal("test", retVals[0])
	require.Equal(5.0, retVals[1])
	require.Equal(true, retVals[2])
	require.IsType(&LocalLuaData{}, retVals[3])

	require.Nil(err)

	require.Len(callbackArgs, 4)
	require.Equal(5.0, callbackArgs[0])
	require.Equal("bleh", callbackArgs[1])
	require.Nil(callbackArgs[2])
	require.IsType(&LocalLuaData{}, callbackArgs[3])

	require.Equal(0, outlyingAllocs())
}

func BenchmarkFib35(b *testing.B) {
	clearAllocs()
	vm := NewState()
	defer vm.Close()

	err := vm.DoString(`
function fib(val)
	if val < 2 then 
		return val
	end

	return fib(val-2) + fib(val-1)
end
`)
	if err != nil {
		panic(err)
	}

	funcObj, err := vm.GetGlobal("fib")
	if err != nil {
		panic(err)
	}

	f := funcObj.(*LocalLuaFunction)
	out, err := f.Call(35)
	if err != nil {
		panic(err)
	}
	fmt.Println(out[0])
}

var cbCount = 0

func AddCallback(args []interface{}) ([]interface{}, error) {
	cbCount++
	if len(args) != 2 {
		return nil, errors.New("incorrect arguments passed to Add")
	}

	l, ok := args[0].(float64)
	if !ok {
		return nil, errors.New("argument 1 to Add was not a number")
	}
	r, ok := args[1].(float64)
	if !ok {
		return nil, errors.New("argument 2 to Add was not a number")
	}

	return []interface{}{
		l + r,
	}, nil
}

func BenchmarkCallbackFib35(b *testing.B) {
	clearAllocs()
	vm := NewState()
	defer vm.Close()

	err := vm.SetGlobal("_Add", LuaCallback(AddCallback))
	if err != nil {
		panic(err)
	}

	err = vm.DoString(`
function fib(val)
	if val < 2 then 
		return val
	end

	return _Add(fib(val-2), fib(val-1))
end
`)
	if err != nil {
		panic(err)
	}

	funcObj, err := vm.GetGlobal("fib")
	if err != nil {
		panic(err)
	}

	b.ResetTimer()

	f := funcObj.(*LocalLuaFunction)
	out, err := f.Call(35)
	if err != nil {
		panic(err)
	}
	fmt.Println(cbCount)
	fmt.Println(out[0])
}
