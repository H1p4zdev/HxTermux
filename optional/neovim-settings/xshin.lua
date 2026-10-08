-- Small HxTermux defaults layered on top of the NvChad starter.
vim.opt.wrap = false
vim.opt.number = true
vim.opt.guicursor = ""

vim.api.nvim_create_autocmd("BufReadPost", {
  desc = "Restore the last cursor position when reopening a file",
  callback = function(event)
    local mark = vim.api.nvim_buf_get_mark(event.buf, '"')
    local line = mark[1]
    if line > 1 and line <= vim.api.nvim_buf_line_count(event.buf) then
      vim.api.nvim_win_set_cursor(0, mark)
    end
  end,
})

local map = vim.keymap.set
local opts = { silent = true }

map("n", "<C-r>", function()
  vim.cmd("vnew")
  vim.cmd("terminal")
end, opts)
map("n", "<C-x>", function()
  vim.cmd("10new")
  vim.cmd("terminal")
end, opts)
map("n", "<M-t>", "<Cmd>terminal<CR>", opts)
map({ "n", "i", "v" }, "<C-s>", "<Esc><Cmd>write<CR>", opts)
map({ "n", "i", "v" }, "<C-q>", "<Esc><Cmd>quit<CR>", opts)
