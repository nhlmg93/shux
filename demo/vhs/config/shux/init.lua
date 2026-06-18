shux.opt.bind = "127.0.0.1:23299"
shux.opt.shell = "/bin/bash"

local root = os.getenv("SHUX_DEMO_ROOT")
if root and root ~= "" then
  shux.opt.state_dir = root .. "/demo/vhs/state"
end
