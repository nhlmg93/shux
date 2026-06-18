shux.opt.shell = "/bin/sh"
shux.opt.bind = "127.0.0.1:23234"
shux.opt.scrollback = 10000
shux.opt.journal_max_mb = 64
shux.opt.journal_replay_delay_ms = 200
shux.opt.resurrection = true

local state = shux.fn.stdpath("state")
if state and state ~= "" then
  shux.opt.state_dir = state
end
