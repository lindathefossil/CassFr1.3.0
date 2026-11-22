--!strict
local ScriptEditorService = game:GetService("ScriptEditorService")

local Applier = {}
Applier.__index = Applier

export type IdPair = {
    newId: number,
    oldId: number
}

type Setter = (instance: any, oldId: number, newId: number) -> ()

export type Applier = typeof(setmetatable({} :: {
    _filteredIds: { [number]: { Instance } },
    _setters: { [string]: Setter },
    _pending: { IdPair },
    _scriptPending: { [Instance]: { [string]: number } },
    _running: boolean,
    _finalizing: boolean,
    _lastScriptFlush: number,
    _lastReport: number,
    _scriptInterval: number,
    _frameBudget: number,
    _applied: number,
    _failed: number,
    _scriptsWritten: number,
}, Applier))

local function idKey(id: number): string
    return string.format("%.0f", id)
end

function Applier.new(filteredIds: { [number]: { Instance } }, setters: any, scriptInterval: number?): Applier
    return setmetatable({
        _filteredIds = filteredIds,
        _setters = setters,
        _pending = {},
        _scriptPending = {},
        _running = false,
        _finalizing = false,
        _lastScriptFlush = os.clock(),
        _lastReport = os.clock(),
        _scriptInterval = scriptInterval or 3,
        _frameBudget = 0.008,
        _applied = 0,
        _failed = 0,
        _scriptsWritten = 0,
    }, Applier)
end

function Applier.replaceTokens(source: string, map: { [string]: number }): (string, boolean)
    local changed = false
    local result = string.gsub(source, "%f[%d]%d+%f[%D]", function(token: string): string?
        local replacement = map[token]
        if not replacement then return nil end
        changed = true
        return idKey(replacement)
    end)
    return result, changed
end

function Applier._writeScript(self: Applier, scriptInstance: Instance, map: { [string]: number })
    if not scriptInstance:IsA("LuaSourceContainer") then return end
    if not scriptInstance:IsDescendantOf(game) then return end

    local readable, source = pcall(function()
        return (scriptInstance :: any).Source :: string
    end)
    if not readable then
        self._failed += 1
        return
    end

    local newSource, changed = Applier.replaceTokens(source, map)
    if not changed then return end

    local written = pcall(function()
        ScriptEditorService:UpdateSourceAsync(scriptInstance, function()
            return newSource
        end)
    end)
    if written then
        self._scriptsWritten += 1
    else
        self._failed += 1
    end
end

function Applier._flushScripts(self: Applier)
    local batch = self._scriptPending
    self._scriptPending = {}
    self._lastScriptFlush = os.clock()

    local started = os.clock()
    for scriptInstance, map in batch do
        self:_writeScript(scriptInstance, map)
        if os.clock() - started > self._frameBudget then
            task.wait()
            started = os.clock()
        end
    end
end

function Applier._applyBatch(self: Applier, batch: { IdPair })
    local started = os.clock()

    for _, idPair in batch do
        local oldId = idPair.oldId
        local newId = idPair.newId
        local instances = self._filteredIds[oldId]

        if instances then
            for _, instance in instances do
                if instance:IsA("LuaSourceContainer") then
                    local map = self._scriptPending[instance]
                    if not map then
                        map = {}
                        self._scriptPending[instance] = map
                    end
                    map[idKey(oldId)] = newId
                    continue
                end

                local setter = self._setters[instance.ClassName]
                if not setter then
                    self._failed += 1
                    continue
                end

                local success, result = pcall(setter, instance, oldId, newId)
                if not success then
                    self._failed += 1
                    if self._failed <= 5 then
                        warn("CassFr: failed to change", instance, `{oldId} to {newId}:`, result)
                    end
                end

                if os.clock() - started > self._frameBudget then
                    task.wait()
                    started = os.clock()
                end
            end
        end

        self._applied += 1
    end
end

function Applier._report(self: Applier, force: boolean)
    if not force and os.clock() - self._lastReport < 5 then return end
    self._lastReport = os.clock()

    local message = `CassFr: applied {self._applied} ids`
    if self._failed > 0 then
        message ..= `, {self._failed} failed`
    end
    print(message)
end

function Applier._run(self: Applier)
    while true do
        local batch = self._pending
        if #batch > 0 then
            self._pending = {}
            self:_applyBatch(batch)
        end

        local hasScripts = next(self._scriptPending) ~= nil
        local due = os.clock() - self._lastScriptFlush >= self._scriptInterval
        if hasScripts and (due or self._finalizing) then
            self:_flushScripts()
        end

        self:_report(false)

        if #self._pending == 0 and next(self._scriptPending) == nil then
            break
        end

        if #self._pending == 0 then
            task.wait(0.1)
        end
    end

    self._running = false
end

function Applier.Add(self: Applier, idPairs: { IdPair })
    for _, idPair in idPairs do
        table.insert(self._pending, idPair)
    end

    if self._running or #self._pending == 0 then return end
    self._running = true
    task.spawn(Applier._run, self)
end

function Applier.Pending(self: Applier): number
    return #self._pending
end

function Applier.Applied(self: Applier): number
    return self._applied
end

function Applier.Finish(self: Applier)
    self._finalizing = true

    if not self._running and (#self._pending > 0 or next(self._scriptPending) ~= nil) then
        self._running = true
        task.spawn(Applier._run, self)
    end

    while self._running do
        task.wait(0.05)
    end

    self:_report(true)
    self._finalizing = false
end

return Applier
