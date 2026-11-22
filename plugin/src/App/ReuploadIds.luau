--!strict
local AssetIdFilter = require("./AssetIdFilter")
local Chunker = require("./Chunker")
local Connection = require("./Connection")
local ConvertWorker = require("./ConvertWorker")
local retry = require("./Retry")
local StatusCodes = require("./StatusCodes")
local UiLibrary = require("../UiLibrary")

local MarketplaceService = game:GetService("MarketplaceService")
local ScriptEditorService = game:GetService("ScriptEditorService")
local HttpService = game:GetService("HttpService")

local currentVersion = script.Parent.Parent.Parent.Version.Value
local busy = false

type ProductInfo = {
    AssetTypeId: number,
    TargetId: number,
    IsNew: boolean,
    Updated: string,
    AssetId: number,
    ProductId: number,
    MinimumMembershipLevel: number,
    Created: string,
    Creator: {
        Name: string,
        CreatorTargetId: number,
        Id: number,
        HasVerifiedBadge: boolean,
        CreatorType: string
    },
    IsLimited: boolean,
    ContentRatingTypeId: number,
    Sales: number,
    IsPublicDomain: boolean,
    Name: string,
    IsForSale: boolean,
    Description: string,
    IconImageAssetId: number,
    IsLimitedUnique: boolean
}

type Request = {
	placeId: number,
	creatorId: number,
	ids: { number },
	defaultPlaceIds: { number },
	pluginVersion: string,
	assetType: string,
	exportJson: boolean,
	isGroup: boolean,
	chunkIndex: number,
	chunkCount: number,
	chunkSizes: { number },
	totalIds: number,
	animationConvert: string
}

local function scriptInjectionEnabled()
    local testScript = Instance.new("Script")
	local success = pcall(ScriptEditorService.UpdateSourceAsync, ScriptEditorService, testScript, function() 
		return "" 
	end)
	
	testScript:Destroy()
	return success
end

function getGameInfo(): (boolean, number, number)
    local creatorId, placeId = game.CreatorId, game.PlaceId

    if placeId == 0 then
        return false, creatorId, placeId 
    end

    if creatorId ~= 0 then
        local isGroup =  game.CreatorType == Enum.CreatorType.Group
        return isGroup, creatorId, placeId
    end

    local success, result = retry(3, MarketplaceService.GetProductInfo, MarketplaceService, placeId)
    if not success then
        return false, creatorId, placeId
    end

    local creatorInfo = (result :: ProductInfo).Creator
    local isGroup = creatorInfo.CreatorType == "Group"
    return isGroup, creatorInfo.CreatorTargetId, placeId
end

return function(plugin: Plugin, ui: UiLibrary.Ui, filterOptions: AssetIdFilter.FilterOptions, assetType: "Animation" | "Sound" | "Mesh", placeArray: { number }?)
    if busy then
        ui:Notify("Notification", "Plugin is busy.")
        return 
    end
    busy = true

    if not scriptInjectionEnabled() then
        ui:Notify("Notification", "Plugin needs script injection permission.")
        busy = false
        return
    end

    local isGroup, creatorId, placeId = getGameInfo()
    if placeId == 0 then
        ui:Notify("Notification", "Game needs to be published. Restart place after publishing.")
        busy = false
        return
    elseif creatorId == 0 then
        ui:Notify("Notification", "Failed to get place info. Try again or restart the place.")
        busy = false
        return
    end
    
    local filteredIds = AssetIdFilter.filterInstances(filterOptions)
    if next(filteredIds) == nil then
        ui:Notify("Notification", "No ids found.")
        busy = false
        return
    end

    local pollSeconds = 1
    local settingsFetched, settingsBody = pcall(HttpService.GetAsync, HttpService, `http://localhost:{plugin:GetSetting("Port")}/settings`)
    if settingsFetched then
        local decoded, settingsData = pcall(HttpService.JSONDecode, HttpService, settingsBody)
        if decoded and type(settingsData) == "table" and type(settingsData.settings) == "table" then
            local pollMs = settingsData.settings.pollMs
            if type(pollMs) == "number" then
                pollSeconds = pollMs / 1000
            end
        end
    end

    local connection, response = Connection.new(plugin:GetSetting("Port"), pollSeconds)
    if not connection then
        ui:Notify("Notification", "Unable to connect to localhost. Did you start the executable?" )
        busy = false
        return
    end

    local convertMode: string? = nil
    if assetType == "Animation" then
        local saved = plugin:GetSetting("AnimationConvert")
        if saved == "r15_to_r6" or saved == "r6_to_r15" then
            convertMode = saved
        end
    end

    local idArray = AssetIdFilter.getIdArray(filteredIds)
    local request: Request = {
        pluginVersion = currentVersion,
        creatorId = creatorId,
        isGroup = isGroup,
        assetType = assetType,
        ids = {},
        placeId = placeId,
        defaultPlaceIds = placeArray or {},
        exportJson = plugin:GetSetting("ExportJSON"),
        chunkIndex = 1,
        chunkCount = 1,
        chunkSizes = {},
        totalIds = #idArray,
        animationConvert = convertMode or ""
    }

    local chunks = Chunker.split(idArray, #HttpService:JSONEncode(request) + 4096)
    local chunkCount = #chunks
    local chunkSizes = {}
    for index, chunk in chunks do
        chunkSizes[index] = #chunk
    end
    request.chunkCount = chunkCount
    request.chunkSizes = chunkSizes

    local currentChunk = 0

    local applier = AssetIdFilter.createApplier(filteredIds)
    local finishing = false
    local convertWorker: { Destroy: (any) -> () }? = nil

    local function finish(message: string?)
        if finishing then return end
        finishing = true
        connection:Destroy()
        if convertWorker then convertWorker:Destroy() end

        task.spawn(function()
            if applier:Pending() > 0 or message == "Finished reuploading all ids." then
                ui:SetStatus("applying ids")
            end
            applier:Finish()

            if message then ui:Notify("Notification", message) end
            ui:SetStatus(nil)
            busy = false
        end)
    end

    local function sendChunk(index: number): boolean
        request.chunkIndex = index
        request.ids = chunks[index]

        local body = HttpService:JSONEncode(request)
        if #body > Chunker.hardLimitBytes then
            finish(`Chunk {index}/{chunkCount} is too large to send ({#body} bytes).`)
            return false
        end

        local success, statusCode = connection:Send("/reupload", body)
        if not success then
            local message: string = (StatusCodes :: any)[statusCode] or ("Unable to send data to localhost.\n" .. statusCode)
            if chunkCount > 1 then
                message = `Chunk {index}/{chunkCount} failed.\n{message}`
            end
            finish(message)
            return false
        end

        currentChunk = index
        if chunkCount > 1 then
            ui:SetStatus(`chunk {index}/{chunkCount}`)
            warn(`CassFr: sent chunk {index}/{chunkCount} ({#chunks[index]} ids)`)
        end
        return true
    end

    local function onDataRecieved(data: string)
        local success, result = pcall(HttpService.JSONDecode, HttpService, data)
        if not success then
            if data == "failed" then
                finish("Reuploading failed to start. Check the executable window.")
                return
            end

            if data ~= "done" then return end

            if currentChunk < chunkCount then
                warn(`CassFr: chunk {currentChunk}/{chunkCount} finished`)
                task.spawn(sendChunk, currentChunk + 1)
                return
            end

            finish("Finished reuploading all ids.")
            return
        end

        if #(result :: any) == 0 then return end
        applier:Add(result :: any)
    end

    connection.onDataRecieved = onDataRecieved
    connection.onDisconnect = function()
        finish("Disconnected from localhost. (Try reconnecting)")
    end

    if convertMode then
        convertWorker = ConvertWorker.start(plugin:GetSetting("Port"), pollSeconds)
    end

    if not sendChunk(1) then return end

    if chunkCount > 1 then
        ui:Notify("Notification", `Sent {#idArray} ids in {chunkCount} chunks.\nOne chunk at a time.\nDashboard: http://localhost:{plugin:GetSetting("Port")}/ui`)
    else
        ui:Notify("Notification", `Sent {#idArray} ids to localhost.`)
    end

    if response and response ~= "done" and response ~= "failed" then
        onDataRecieved(response)
    end
end
