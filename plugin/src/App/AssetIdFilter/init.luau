--!strict
local AssetIdFilter = {}

local getAssetIds = require("./AssetIdFilter/GetAssetIds")
local changeIds = require("./AssetIdFilter/ChangeIds")
local Applier = require("./Applier")

export type FilterOptions = getAssetIds.FilterOptions

function AssetIdFilter.filterInstances(filterOptions: getAssetIds.FilterOptions): { [number]: { Instance } }
    return getAssetIds(filterOptions)
end

function AssetIdFilter.getIdArray(filteredIds: { [number]: { Instance } } ): { number }
    local idArray = {}
    for id, _ in filteredIds do
        table.insert(idArray, id)
    end
    return idArray
end

function AssetIdFilter.replaceIds(filteredIds: { [number]: { Instance } }, idsToChange: { changeIds.IdPair })
    changeIds.apply(filteredIds, idsToChange)
end

function AssetIdFilter.createApplier(filteredIds: { [number]: { Instance } }): Applier.Applier
    return Applier.new(filteredIds, changeIds.setters)
end

return AssetIdFilter
