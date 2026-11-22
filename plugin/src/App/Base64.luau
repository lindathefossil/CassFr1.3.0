--!nonstrict
-- Small base64 encoder/decoder working on buffers. Used to move animation data
-- between CassFr.exe (JSON) and the plugin (buffers).
local Base64 = {}

local alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
local encodeMap: { [number]: number } = {}
local decodeMap: { [number]: number } = {}

for i = 0, 63 do
	local byte = string.byte(alphabet, i + 1)
	encodeMap[i] = byte
	decodeMap[byte] = i
end

function Base64.decode(text: string): buffer
	local length = #text
	local padding = 0
	if length >= 1 and string.byte(text, length) == 61 then padding += 1 end
	if length >= 2 and string.byte(text, length - 1) == 61 then padding += 1 end

	local outLength = (length // 4) * 3 - padding
	local out = buffer.create(math.max(outLength, 0))

	local o = 0
	for i = 1, length - 3, 4 do
		local a = decodeMap[string.byte(text, i)] or 0
		local b = decodeMap[string.byte(text, i + 1)] or 0
		local c = decodeMap[string.byte(text, i + 2)] or 0
		local d = decodeMap[string.byte(text, i + 3)] or 0
		local n = a * 262144 + b * 4096 + c * 64 + d

		if o < outLength then buffer.writeu8(out, o, n // 65536) end
		if o + 1 < outLength then buffer.writeu8(out, o + 1, (n // 256) % 256) end
		if o + 2 < outLength then buffer.writeu8(out, o + 2, n % 256) end
		o += 3
	end

	return out
end

function Base64.encode(data: buffer): string
	local length = buffer.len(data)
	local pieces = table.create(length // 3 + 1)

	local i = 0
	while i < length do
		local b1 = buffer.readu8(data, i)
		local b2 = if i + 1 < length then buffer.readu8(data, i + 1) else nil
		local b3 = if i + 2 < length then buffer.readu8(data, i + 2) else nil

		local n = b1 * 65536 + (b2 or 0) * 256 + (b3 or 0)
		pieces[#pieces + 1] = string.char(
			encodeMap[n // 262144],
			encodeMap[(n // 4096) % 64],
			if b2 then encodeMap[(n // 64) % 64] else 61,
			if b3 then encodeMap[n % 64] else 61
		)
		i += 3
	end

	return table.concat(pieces)
end

return Base64
