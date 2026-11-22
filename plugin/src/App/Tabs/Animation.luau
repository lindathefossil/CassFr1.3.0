--!strict
local getFilterOptions = require("../GetFilterOptions")
local reuploadIds = require("../ReuploadIds")
local UiLibrary = require("../../UiLibrary")

local Selection = game:GetService("Selection")

return function(ui: UiLibrary.Ui, plugin: Plugin)
	local tab = ui:CreateTab("Animation")

	-- Optional R15 <-> R6 conversion while reuploading. Only one direction can be on.
	local r15ToR6: any = nil
	local r6ToR15: any = nil

	local function setMode(mode: string, enabled: boolean)
		if enabled then
			plugin:SetSetting("AnimationConvert", mode)
		elseif plugin:GetSetting("AnimationConvert") == mode then
			plugin:SetSetting("AnimationConvert", "")
		end
	end

	local saved = plugin:GetSetting("AnimationConvert")
	r15ToR6 = tab:CreateToggle("Convert R15 -> R6 while reuploading", function(state)
		if state and r6ToR15 and r6ToR15.state then
			r6ToR15:Toggle()
		end
		setMode("r15_to_r6", state)
	end, saved == "r15_to_r6")
	r6ToR15 = tab:CreateToggle("Convert R6 -> R15 while reuploading", function(state)
		if state and r15ToR6 and r15ToR6.state then
			r15ToR6:Toggle()
		end
		setMode("r6_to_r15", state)
	end, saved == "r6_to_r15")
	tab:CreateLabel("Converting needs this plugin open in Studio until the reupload finishes.")

	tab:CreateButton("Reupload", function()
		local filter = getFilterOptions(plugin, game:GetDescendants())
		table.insert(filter.WhitelistedInstances, "Animation")
		reuploadIds(plugin, ui, filter, "Animation", _G.getPlaceList()) -- forgive me
	end)
	
	tab:CreateButton("Reupload Selected", function()
		local filter =  {
			WhitelistedInstances = { "Animation", "StringValue", "NumberValue", "IntValue", "LuaSourceContainer" },
			Instances = Selection:Get()
		}

		reuploadIds(plugin, ui, filter, "Animation", _G.getPlaceList()) -- forgive me
	end)
end
