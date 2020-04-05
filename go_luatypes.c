#include "go_luajit.h"

void free_table_entry(lua_State *L, lua_table_entry *entry) {
    if (entry == NULL)
        return;

    free_lua_value(L, entry->key);
    entry->key = NULL;
    free_lua_value(L, entry->value);
    entry->value = NULL;
    chfree(entry);
}

void free_unrolled_table(lua_State *L, lua_unrolled_table *table) {
    table->last = NULL;
    lua_table_entry *next = table->first;
    while (next != NULL) {
        table->first = next->next;
        free_table_entry(L, next);
        next = table->first;
    }
    table->first = NULL;
}

void free_lua_value(lua_State *L, lua_value *value) {
    if (value == NULL)
        return;

    switch(value->valueType) {
        case LUA_TSTRING:
            chfree(value->data.pointerVal);
            break;
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
        case LUA_TUNROLLEDTABLE:
            free_unrolled_table(L, (lua_unrolled_table*)value->data.pointerVal);
            break;
        default:
            break;
    }
    chfree(value);
}

void free_lua_return(lua_State *_L, lua_return retVal, _Bool freeValues) {
    if (retVal.err != NULL)
        free_lua_error(retVal.err);
    
    if (freeValues) {
        for (int i = 0; i < retVal.valueCount; i++) {
            free_lua_value(_L, retVal.values[i]);
        }
    }

    if (retVal.valueCount > 0) {
        chfree(retVal.values);
    }
}

void free_lua_args(lua_State *_L, lua_args args, _Bool freeValues) {
    if (freeValues) {
        for (int i = 0; i < args.valueCount; i++) {
            free_lua_value(_L, args.values[i]);
        }
    }

    chfree(args.values);
}

lua_result unroll_table(lua_State *_L, int luaRefVal) {
    lua_result retVal;
    lua_unrolled_table *unrolled = chmalloc(sizeof(lua_unrolled_table));
    retVal.err = NULL;
    retVal.value = chmalloc(sizeof(lua_value));
    retVal.value->valueType = LUA_TUNROLLEDTABLE;
    retVal.value->data.pointerVal = NULL;

    //Push rolled table to top of stack
    lua_rawgeti(_L, LUA_REGISTRYINDEX, luaRefVal);

    unrolled->first = NULL;
    unrolled->last = NULL;

    lua_pushnil(_L); // First key to start iteration
    while (lua_next(_L, -1) != 0) {

        lua_result key;
        lua_result value = convert_stack_value(_L);

        if (value.err == NULL) {
            key = convert_stack_value_impl(_L, 1);
        }


        if (value.err != NULL || key.err != NULL) {
            //Move error over to output
            if (value.err != NULL) {
                retVal.err = value.err;
            } else {
                retVal.err = key.err;
            }

            //Free allocated values
            if (key.value != NULL) free_lua_value(_L, key.value);
            if (value.value != NULL) free_lua_value(_L, value.value);

            //Free the unrolled table in progress
            free_unrolled_table(_L, unrolled);
            retVal.value = NULL;

            //Pop table from stack
            lua_pop(_L, 1);

            return retVal;
        }

        lua_table_entry *entry = chmalloc(sizeof(lua_table_entry));
        entry->key = key.value;
        entry->value = value.value;
        entry->next = NULL;

        if (unrolled->first == NULL)  {
            unrolled->first = entry;
            unrolled->last = entry;
        } else {
            unrolled->last->next = entry;
            unrolled->last = entry;
        }
     }

     retVal.value->data.pointerVal = unrolled;

     //remove table to cleanup
     lua_pop(_L, 1);

     return retVal;
}

_Bool isUData(lua_State *_L, const char *name) {
    luaL_getmetatable(_L, name);
    int equal = lua_rawequal(_L, -1, -2);
    lua_pop(_L, 1);
    return (_Bool)equal;
}

lua_result convert_stack_value(lua_State *L) {
    return convert_stack_value_impl(L, 0);
}

lua_result convert_stack_value_impl(lua_State *L, _Bool suppressPop) {
    int type = lua_type(L, -1);
    lua_result retVal = {};
    retVal.err = NULL;
    retVal.value = NULL;

    if (type == LUA_TNIL) {
        lua_pop(L, 1);
        return retVal;
    }

    retVal.value = chmalloc(sizeof(lua_value));
    retVal.value->valueType = type;
    retVal.value->dataArg.isCFunction = 0;
    retVal.value->data.pointerVal = 0;
    retVal.err = NULL;
    _Bool needsPop = !suppressPop;

    switch(type) {
        case LUA_TNUMBER:
            retVal.value->data.numberVal = (double)lua_tonumber(L, -1);
            break;
        case LUA_TBOOLEAN:
            retVal.value->data.booleanVal = (_Bool)lua_toboolean(L, -1);
            break;
        case LUA_TSTRING:
            {
                const char *luaStr = lua_tolstring(L, -1, &(retVal.value->dataArg.stringLen));
                char *outStr = chmalloc(sizeof(char)*(retVal.value->dataArg.stringLen+1));
                strncpy(outStr, luaStr, retVal.value->dataArg.stringLen+1);
                retVal.value->data.pointerVal = (void*)outStr;
                break;
            }
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
            {
                if (type == LUA_TUSERDATA) {
                    //For UData's we should try to provide the type to give golang an easier time
                    retVal.value->dataArg.userDataType = 0;
                    int gotMeta = lua_getmetatable(L, -1);
                    if (gotMeta) {
                        if (isUData(L, MT_GOCALLBACK))
                            retVal.value->dataArg.userDataType = META_GOCALLBACK;
                        lua_pop(L, 1);
                    }
                }

                //Intentional fallthrough
            }
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
    retVal.values = chmalloc(valueCount * sizeof(lua_value*));
    for (int i = 0; i < valueCount; i++) {
        lua_result result = convert_stack_value(_L);
        if (result.err != NULL) {
            //Just return error- free all allocations made until this point
            retVal.err = result.err;
            for (int j = 0; j < i; j++) {
                free_lua_value(_L, retVal.values[valueCount-j-1]);
            }
            chfree(retVal.values);
            retVal.values = NULL;
            retVal.valueCount = 0;
            return retVal;
        }

        retVal.values[valueCount-i-1] = result.value;
    }

    return retVal;
}

lua_err *push_unrolled_table(lua_State *_L, lua_unrolled_table *table) {
    lua_newtable(_L);
    lua_table_entry *next = table->first;
    while (next != NULL) {
        lua_err *err = push_lua_value(_L, next->key);
        if (err != NULL) return err;

        err = push_lua_value(_L, next->value);
        if (err != NULL) return err;

        lua_rawset(_L, -3);

        next = next->next;
    }

    return NULL;
}

lua_err *push_lua_value(lua_State *_L, lua_value *value) {
    if (value == NULL) {
        lua_pushnil(_L);
        return NULL;
    }
    
    switch(value->valueType) {
        case LUA_TUNLOADEDCALLBACK:
            {
                //This came from golang, it's a cgo handle for a go function
                void **userData = (void**)lua_newuserdata(_L, sizeof(void*));
                *userData = value->data.pointerVal;
                luaL_getmetatable(_L, MT_GOCALLBACK);
                lua_setmetatable(_L, -2);
                break;
            }
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
        case LUA_TUNROLLEDTABLE:
            return push_unrolled_table(_L, (lua_unrolled_table*)value->data.pointerVal);
        default:
            return create_lua_error("CANNOT PUSH TO STACK - INVALID VALUE");
    }

    return NULL;
}

lua_err *push_lua_args(lua_State *_L, lua_args args) {
    int alreadyPushed = 0;
    for (int i = 0; i < args.valueCount; i++) {
        lua_err *err = push_lua_value(_L, args.values[args.valueCount-i-1]);
        if (err != NULL) {
            if (alreadyPushed > 0)
                lua_pop(_L, alreadyPushed);
            return err;
        }
        alreadyPushed++;
    }

    return NULL;
}

lua_err *push_lua_return(lua_State *_L, lua_return retVal) {
    int alreadyPushed = 0;
    for (int i = 0; i < retVal.valueCount; i++) {
        lua_err *err = push_lua_value(_L, retVal.values[i]);
        if (err != NULL) {
            if (alreadyPushed > 0)
                lua_pop(_L, alreadyPushed);
            return err;
        }
        alreadyPushed++;
    }

    return NULL;
}

lua_value **build_values(int slots, int allocs) {
    lua_value** valueList = chmalloc(sizeof(lua_value*)*slots);
    for (int i = 0; i < allocs; i++) {
        valueList[i] = chmalloc(sizeof(lua_value));
    }
    return valueList;
}