--!strict
local Chunker = {}

Chunker.maxBodyBytes = 900000
Chunker.hardLimitBytes = 1040000

local slackBytes = 32

local function idBytes(id: number): number
    return #string.format("%.0f", id) + 1
end

function Chunker.split(ids: { number }, reservedBytes: number): { { number } }
    local budget = math.max(Chunker.maxBodyBytes - reservedBytes, 1024)

    local totalBytes = 0
    for _, id in ids do
        totalBytes += idBytes(id)
    end

    if totalBytes == 0 then
        return {}
    end

    local chunkCount = math.ceil(totalBytes / budget)
    local target = math.ceil(totalBytes / chunkCount) + slackBytes

    local chunks: { { number } } = {}
    local current: { number } = {}
    local currentBytes = 0

    for _, id in ids do
        local bytes = idBytes(id)
        if currentBytes + bytes > target and #current > 0 then
            table.insert(chunks, current)
            current = {}
            currentBytes = 0
        end

        table.insert(current, id)
        currentBytes += bytes
    end

    if #current > 0 then
        table.insert(chunks, current)
    end

    return chunks
end

return Chunker
