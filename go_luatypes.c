#include "go_luajit.h"

void free_lua_value(lua_State *L, lua_value *value) {
    if (value == NULL)
        return;

    switch(value->valueType) {
        case LUA_TFUNCTION:
            if (value->dataArg.isCFunction)
                break;
            //Intentionally falling through- lua functions need to be stored as refs
        case LUA_TUSERDATA:
        case LUA_TTHREAD:
        case LUA_TLIGHTUSERDATA:
        case LUA_TTABLE:
            luaL_unref(L, LUA_REGISTRYINDEX, value->data.luaRefVal);
            break;
        default:
            break;
    }
    free(value);
}

void free_lua_return(lua_State *_L, lua_return retVal, _Bool freeValues) {
    if (retVal.err != NULL)
        free_lua_error(retVal.err);
    
    if (freeValues) {
        for (int i = 0; i < retVal.valueCount; i++) {
            free_lua_value(_L, retVal.values[i]);
        }
    }

    free(retVal.values);
}

void free_lua_args(lua_State *_L, lua_args args, _Bool freeValues) {
    if (freeValues) {
        for (int i = 0; i < args.valueCount; i++) {
            free_lua_value(_L, args.values[i]);
        }
    }

    free(args.values);
}

lua_result convert_stack_value(lua_State *L) {
    int type = lua_type(L, -1);
    lua_result retVal = {};

    if (type == LUA_TNIL) {
        lua_pop(L, 1);
        return retVal;
    }

    retVal.value = malloc(sizeof(lua_value));
    retVal.value->valueType = type;
    retVal.value->dataArg.isCFunction = 0;
    retVal.value->data.pointerVal = 0;
    retVal.err = NULL;
    _Bool needsPop = 1;
    
    switch(type) {
        case LUA_TNUMBER:
            retVal.value->data.numberVal = (double)lua_tonumber(L, -1);
            break;
        case LUA_TBOOLEAN:
            retVal.value->data.booleanVal = (_Bool)lua_toboolean(L, -1);
            break;
        case LUA_TSTRING:
            retVal.value->data.pointerVal = (void*)lua_tolstring(L, -1, &(retVal.value->dataArg.stringLen));
            break;
        case LUA_TFUNCTION:
            {
                if (lua_iscfunction(L, -1)) {
                    retVal.value->dataArg.isCFunction = 1;
                    retVal.value->data.pointerVal = (void*)lua_tocfunction(L, -1);
                    break;
                }
                //Intentionally falling through- lua functions need to be stored as refs
            }
        case LUA_TUSERDATA:
        case LUA_TTHREAD:
        case LUA_TLIGHTUSERDATA:
        case LUA_TTABLE:
            retVal.value->data.luaRefVal = luaL_ref(L, LUA_REGISTRYINDEX);
            needsPop = 0;
            break;
        default:
            retVal.err = create_lua_error("CANNOT POP FROM STACK - INVALID STACK VALUE");
            needsPop = 0;
            break;
    }

    if (needsPop)
        lua_pop(L, 1);

    return retVal;
}

lua_return pop_lua_values(lua_State *_L, int valueCount) {
    lua_return retVal = {};
    retVal.valueCount = valueCount;
    retVal.err = NULL;
    retVal.values = malloc(valueCount * sizeof(lua_value*));
    for (int i = 0; i < valueCount; i++) {
        lua_result result = convert_stack_value(_L);
        if (result.err != NULL) {
            //Just return error- free all allocations made until this point
            retVal.err = result.err;
            for (int j = 0; j < i; j++) {
                free_lua_value(_L, retVal.values[j]);
            }
            free(retVal.values);
            retVal.values = NULL;
            retVal.valueCount = 0;
            return retVal;
        }

        retVal.values[i] = result.value;
    }

    return retVal;
}

lua_err *push_lua_value(lua_State *_L, lua_value *value) {
    if (value == NULL) {
        lua_pushnil(_L);
        return NULL;
    }
    
    switch(value->valueType) {
        case LUA_TNUMBER:
            lua_pushnumber(_L, (lua_Number)value->data.numberVal);
            break;
        case LUA_TBOOLEAN:
            lua_pushboolean(_L, (int)value->data.booleanVal);
            break;
        case LUA_TSTRING:
            lua_pushlstring(_L, (const char*)value->data.pointerVal, value->dataArg.stringLen);
            break;
        case LUA_TFUNCTION:
            {
                if (value->dataArg.isCFunction) {
                    lua_pushcfunction(_L, (lua_CFunction)value->data.pointerVal);
                    break;
                }
                //Intentionally falling through- lua functions need to be stored as refs
            }
        case LUA_TUSERDATA:
        case LUA_TTHREAD:
        case LUA_TLIGHTUSERDATA:
        case LUA_TTABLE:
            lua_rawgeti(_L, LUA_REGISTRYINDEX, value->data.luaRefVal);
            break;
        default:
            return create_lua_error("CANNOT PUSH TO STACK - INVALID VALUE");
    }

    return NULL;
}

lua_err *push_lua_args(lua_State *_L, lua_args args) {
    int alreadyPushed = 0;
    for (int i = 0; i < args.valueCount; i++) {
        lua_err *err = push_lua_value(_L, args.values[i]);
        if (err != NULL) {
            if (alreadyPushed > 0)
                lua_pop(_L, alreadyPushed);
            return err;
        }
        alreadyPushed++;
    }

    return NULL;
}
