package luajitter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimpleGlobal(t *testing.T) {
	vm := NewState()
	defer vm.Close()

	err := vm.SetGlobal("wow", 2)
	require.Nil(t, err)

	val, err := vm.GetGlobal("wow")
	require.Nil(t, err)
	require.NotNil(t, val)

	number, ok := val.(float64)
	require.True(t, ok)
	require.Equal(t, 2.0, number)
}

func TestInitGlobal(t *testing.T) {
	vm := NewState()
	defer vm.Close()

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
	vm := NewState()
	defer vm.Close()

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
}

func TestDoStringAndCallNil(t *testing.T) {
	vm := NewState()
	defer vm.Close()

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
}

func TestDoStringWithError(t *testing.T) {
	vm := NewState()
	defer vm.Close()

	err := vm.DoString(`error("some error")`)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "some error")
}

func TestDoCallWithError(t *testing.T) {
	vm := NewState()
	defer vm.Close()

	err := vm.DoString(`function errcall(msg) error(msg) end`)

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
}
