#include "_cgo_export.h"

int release_cgo_handle(lua_State *_L) {
    void **handle = (void**)lua_touserdata(_L, -1);
    lua_err *err = releaseCGOHandle(*handle);
    lua_pop(_L, 1);
    if (err != NULL)
        return raise_lua_error(_L, err);
	return 0;
}

int execute_go_callback(lua_State *_L) {
    lua_args args = {};
    lua_err *err = NULL;
    args.valueCount = lua_gettop(_L)-1;

    if (args.valueCount > 0) {
        lua_return retVal = pop_lua_values(_L, args.valueCount);
        if (retVal.err != NULL) {
            return raise_lua_error(_L, retVal.err);
        }

        args.values = retVal.values;
    }

    void **goCallback = (void**)lua_touserdata(_L, 1);
    lua_pop(_L, 1);

    lua_return *goReturn = callbackGoFunction(_L, *goCallback, args);
    if (goReturn->err != NULL) {
        err = goReturn->err;
        goReturn->err = NULL;
        free_lua_return(_L, *goReturn, 1);
        chfree(goReturn);
        return raise_lua_error(_L, err);
    }

    err = push_lua_return(_L, *goReturn);
    int valueCount = goReturn->valueCount;
    free_lua_return(_L, *goReturn, 1);
    chfree(goReturn);
    if (err != NULL) {
        return raise_lua_error(_L, err);
    }

    return valueCount;
}
