#include "go_luajit.h"

const lua_err INVALID_ERROR_str = {"INVALID ERROR"};
lua_err *INVALID_ERROR = (lua_err *)&INVALID_ERROR_str;

lua_err *get_lua_error(lua_State *_L, int errCode) {
    if (errCode == 0)
        return NULL;
    if (errCode == LUA_ERRMEM)
        return create_lua_error("LUA OUT OF MEMORY");

	const char *message = lua_tolstring(_L, -1, NULL);
	if (message == NULL)
		return INVALID_ERROR;

	lua_pop(_L, 1);
	return create_lua_error_from_luastr(message);
}

lua_err *create_lua_error_from_luastr(const char *msg) {
	lua_err *err = malloc(sizeof(lua_err));
	char *newMessage = malloc(sizeof(char)*(strlen(msg)+1));
	strncpy(newMessage, msg, strlen(msg));
	err->message = newMessage;

	return err;
}

lua_err *create_lua_error(char *msg) {
	lua_err *err = malloc(sizeof(lua_err));
	err->message = msg;

	return err;
}

void free_lua_error(lua_err *err) {
    if (err == NULL)
        return;
	free(err->message);
	err->message = NULL;
    free(err);
}
