-- local regularDbPath = "postgresql://postgres@/classy"
local regularDbPath = "postgresql://postgres@/classytest"
local testDbPath = "postgresql://postgres@/classytest"
vim.g.db = regularDbPath
function LocalSwitchDatabase()
	if vim.g.db == regularDbPath then
		vim.g.db = testDbPath
	else
		vim.g.db = regularDbPath
	end
end

-- update all files for gopls lsp when autogenrated code should be triggered
local files1 = vim.fn.glob("./**/components/**/*.go", true, true)
local files2 = vim.fn.glob("./**/*.sql.go", true, true)
local files = vim.list_extend(files1, files2)
for _, file in ipairs(files) do
	vim.api.nvim_command("let buf=bufnr('%') | e " .. vim.fn.fnameescape(file) .. "| exec 'b' buf")
end

vim.api.nvim_create_autocmd("BufWritePost", {
	pattern = { "*.templ", "*.sql" },
	callback = function()
		vim.defer_fn(function()
			vim.cmd("let buf=bufnr('%') | exec 'bufdo update' | exec 'b' buf")
		end, 60)
	end,
})

-- css completions
-- Project-specific HTML/CSS configuration
vim.g.html_css = {
	-- handlers = {
	-- 	definition = {
	-- 		bind = "gd",
	-- 	},
	-- 	hover = {
	-- 		bind = "K",
	-- 		wrap = true,
	-- 		border = "none",
	-- 		position = "cursor",
	-- 	},
	-- },
	-- documentation = {
	-- 	auto_show = true,
	-- },
	style_sheets = {
		-- "../static/milligram.css",
	},
}
