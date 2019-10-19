#include <stdlib.h>
#include <stdio.h>
#include <luajit.h>
#include <lua.h>
#include <lauxlib.h>
#include <lualib.h>
#include <string.h>
#include "go_diag_memory.h"
#include "go_luaerrors.h"
#include "go_luatypes.h"
#include "go_luainterface.h"

extern lua_err *internal_dostring(lua_State *_L, char *script);
