---@meta
---@diagnostic disable: lowercase-global

-- LuaLS stub for shux's injected Lua config/plugin runtime.
--
-- Runtime notes:
-- - `shux` is injected as a global in config/plugin files.
-- - `require("shux")` resolves to the same runtime object.
-- - This file is for editor completion and type information only.

---@alias shux.OptionName
---| 'prefix'
---| 'shell'
---| 'session_name'
---| 'mouse'

---@alias shux.KeyTableName
---| 'prefix'

---@class shux.Options
---@field prefix string Prefix key, e.g. "C-b"
---@field shell string Default shell path
---@field session_name string Default session name
---@field mouse boolean Whether mouse support is enabled

---@class shux.KeymapAPI
local keymap = {}

---@param table_name shux.KeyTableName
---@param spec string
---@param action string
function keymap.set(table_name, spec, action) end

---@param table_name shux.KeyTableName
---@param spec string
function keymap.del(table_name, spec) end

---@class shux.CommandTable
---@field [string] fun(...: string): boolean, string?

---@class shux.Module
---@field opts shux.Options
---@field keymap shux.KeymapAPI
---@field cmd shux.CommandTable
local shux = {}

---@type shux.Options
shux.opts = {}

---@type shux.KeymapAPI
shux.keymap = keymap

---@type shux.CommandTable
shux.cmd = {}

---@return string
function shux.config_dir() return "" end

---@return string
function shux.config_file() return "" end

---@return string[]
function shux.list_commands() return {} end

---@type shux.Module
_G.shux = shux

return shux
