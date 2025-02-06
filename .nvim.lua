local regularDbPath = "postgresql://postgres@/classy"
local testDbPath = "postgresql://postgres@/classytest"
vim.g.db = regularDbPath
function LocalSwitchDatabase()
	if vim.g.db == regularDbPath then
		vim.g.db = testDbPath
	else
		vim.g.db = regularDbPath
	end
end
-- Create the custom command
vim.cmd("command! SwitchDb lua LocalSwitchDatabase()")
vim.cmd("command! WhichDb echo g:db")
