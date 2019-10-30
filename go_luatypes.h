#define LUA_TUNLOADEDCALLBACK -1

#define META_GOCALLBACK 1

union lua_primitive {
    double numberVal;
    _Bool booleanVal;
    void *pointerVal;
    int luaRefVal;
};
typedef union lua_primitive lua_primitive;

union lua_data_arg {
    _Bool isCFunction;
    size_t stringLen;
    int userDataType;
};
typedef union lua_data_arg lua_data_arg;

struct lua_value {
    int valueType;
    lua_primitive data;
    lua_data_arg dataArg;
};
typedef struct lua_value lua_value;

struct lua_result {
    lua_value *value;
    lua_err *err;
};
typedef struct lua_result lua_result;

struct lua_return {
    int valueCount;
    lua_err *err;
    lua_value **values;
};
typedef struct lua_return lua_return;

struct lua_args {
    int valueCount;
    lua_value **values;
};
typedef struct lua_args lua_args;

extern void free_lua_value(lua_State *L, lua_value *value);
extern void free_lua_return(lua_State *_L, lua_return retVal, _Bool freeValues);
extern void free_lua_args(lua_State *_L, lua_args args, _Bool freeValues);
extern lua_result convert_stack_value(lua_State *L);
extern lua_return pop_lua_values(lua_State *_L, int valueCount);
extern lua_err *push_lua_value(lua_State *_L, lua_value *value);
extern lua_err *push_lua_args(lua_State *_L, lua_args args);
extern lua_err *push_lua_return(lua_State *_L, lua_return retVal);
extern lua_value **build_values(int slots, int allocs);