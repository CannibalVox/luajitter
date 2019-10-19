#include "go_luajit.h"

lua_err *internal_dostring(lua_State *_L, char *script) {
	int retVal = luaL_dostring(_L, script);
	return get_lua_error(_L, retVal);
}
