shux.opt.bind = "127.0.0.1:23299"
shux.opt.shell = "/bin/bash"

local root = os.getenv("SHUX_DEMO_ROOT")
if root and root ~= "" then
  shux.opt.state_dir = root .. "/demo/vhs/state"
end

-- tmux-like split dividers for VHS (no outer window box).
shux.opt.ui = {
  statusline = false,
  pane_border_lines = "single",
  pane_outer_border = false,
  pane_labels = false,
}
