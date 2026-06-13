-- pos-save.lua — mpv script to save playback position for resume
-- Receives media key via --script-opts: ani_pos_key=<anime_title_ep_N>
-- Saves position to ~/.anilix/watch-progress/<key>.pos on shutdown

local mp = require("mp")
local msg = require("mp.msg")

local pos_dir = (os.getenv("HOME") or "/data/data/com.termux/files/home") .. "/.anilix/watch-progress"
local media_key = nil
local saved = false

local function save_position()
    if saved then return end
    if not media_key or media_key == "" then return end

    local pos = mp.get_property_number("time-pos", nil)
    local dur = mp.get_property_number("duration", nil)
    if not pos or not dur or dur == 0 then return end

    -- Don't save if we're at the very beginning or very end
    if pos < 2 or (dur - pos) < 2 then
        msg.info("position too close to start/end, skipping save")
        return
    end

    -- Create directory if needed
    os.execute("mkdir -p \"" .. pos_dir .. "\"")

    local path = pos_dir .. "/" .. media_key .. ".pos"
    local f = io.open(path, "w")
    if f then
        f:write(string.format("%.1f", pos))
        f:close()
        saved = true
        msg.info("saved position " .. string.format("%.1f", pos) .. "s to " .. path)
    else
        msg.warn("failed to write position to " .. path)
    end
end

local function on_file_loaded()
    saved = false
    media_key = mp.get_opt("ani_pos_key")
end

local function on_shutdown()
    save_position()
end

mp.register_event("file-loaded", on_file_loaded)
mp.register_event("shutdown", on_shutdown)
