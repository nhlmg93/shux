local map = shux.keymap.set

map("prefix", "<leader>d", "detach", { desc = "Detach client" })
map("prefix", "<leader>q", "quit", { desc = "Quit when last client" })
map("prefix", "<leader>%", "split_lr", { desc = "Split left/right" })
map("prefix", "<leader>\"", "split_tb", { desc = "Split top/bottom" })
map("prefix", "<leader>o", "next_pane", { desc = "Next pane" })
map("prefix", "<leader>x", "close_pane", { desc = "Close pane" })
map("prefix", "<leader>c", "new_window", { desc = "New window" })
map("prefix", "<leader>n", "next_window", { desc = "Next window" })
map("prefix", "<leader>p", "previous_window", { desc = "Previous window" })
map("prefix", "<leader>?", "list_keymaps", { desc = "List bindings" })

for i = 1, 9 do
  map("prefix", "<leader>" .. i, "select_window_" .. i, { desc = "Window " .. i })
end
map("prefix", "<leader>0", "select_window_10", { desc = "Window 10" })
