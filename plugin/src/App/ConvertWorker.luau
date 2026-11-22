--!nonstrict
-- Talks to CassFr.exe while a reupload runs: asks for animations that need converting,
-- converts them with AnimationConverter and sends the result back.
local HttpService = game:GetService("HttpService")

local AnimationConverter = require("./AnimationConverter")
local Base64 = require("./Base64")

local ConvertWorker = {}

local MAX_JOBS_PER_POLL = 40
-- Roblox lets plugins POST about 1 MB per request. Stay well below it.
local MAX_POST_BYTES = 700 * 1024
local MIN_BUSY_WAIT = 0.5 -- keeps us far below Roblox's 500 requests/minute HttpService limit

local function convertJob(job)
	local result = { id = job.id }

	local ok, data, status, message = pcall(function()
		return AnimationConverter.convertBuffer(Base64.decode(job.data), job.mode)
	end)

	if not ok then
		result.error = "conversion failed: " .. tostring(data)
	elseif status == "skipped" then
		result.skipped = true
		result.message = message
	else
		result.data = Base64.encode(data)
	end

	return result
end

local function post(port: number, results)
	local body = HttpService:JSONEncode(results)
	for attempt = 1, 3 do
		local ok = pcall(HttpService.PostAsync, HttpService, `http://localhost:{port}/convert/result`, body, Enum.HttpContentType.ApplicationJson)
		if ok then return end
		task.wait(attempt)
	end
end

local function process(port: number, jobs)
	local batch, batchBytes = {}, 0

	local function flush()
		if #batch == 0 then return end
		post(port, batch)
		batch, batchBytes = {}, 0
	end

	for index, job in jobs do
		local result = convertJob(job)

		local size = #(result.data or "") + 128
		if size > MAX_POST_BYTES then
			result = { id = job.id, error = "converted animation is too big to send back through the plugin (1 MB request limit)" }
			size = 256
		end

		if batchBytes + size > MAX_POST_BYTES then
			flush()
		end
		table.insert(batch, result)
		batchBytes += size

		if index % 3 == 0 then
			task.wait() -- keep Studio responsive
		end
	end

	flush()
end

function ConvertWorker.start(port: number, interval: number)
	local worker = { _running = true }

	function worker:Destroy()
		self._running = false
	end

	task.spawn(function()
		while worker._running do
			local ok, body = pcall(HttpService.GetAsync, HttpService, `http://localhost:{port}/convert/next?max={MAX_JOBS_PER_POLL}`)

			local handled = 0
			if ok then
				local decoded, jobs = pcall(HttpService.JSONDecode, HttpService, body)
				if decoded and type(jobs) == "table" and #jobs > 0 then
					handled = #jobs
					process(port, jobs)
				end
			else
				task.wait(2)
			end

			task.wait(if handled > 0 then MIN_BUSY_WAIT else math.max(interval, 0.25))
		end
	end)

	return worker
end

return ConvertWorker
