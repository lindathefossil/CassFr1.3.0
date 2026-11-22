--!nonstrict
-- R15 <-> R6 animation converter.
--
-- Based on the idea of Andree's "R15 to R6 Animation Converter" plugin, but it works on
-- keyframe data directly instead of posing a rig in the workspace, so it can run on
-- thousands of animations without touching the place:
--   1. build a track per joint and sample the whole animation (easing included) at a steady rate
--   2. forward kinematics on the source rig (poses -> world rotation of every part)
--   3. pick the world rotation each target part should have
--   4. solve the target rig's joints (world rotation -> poses)
-- Resampling is what keeps the result smooth: R15 and R6 do not interpolate between keyframes
-- the same way, so converting only the original keyframes makes the motion drift and look choppy.
--
-- Only rotations are used, so the joints' offsets (translations) don't matter. The one
-- translation that is carried over is the root pose (hip bobbing, crouching, ...).
--
-- The conversion is an approximation: R6 has no elbows, knees, hands or feet and R15 has
-- no single "arm" part, so limbs are blended / copied. See ARM_BLEND and LEG_BLEND.

local SerializationService = game:GetService("SerializationService")
local TweenService = game:GetService("TweenService")

local AnimationConverter = {}

-- R15 -> R6: how much of the lower limb's rotation is mixed into the single R6 limb.
-- 0 = only the upper arm/leg (shoulder/hip), 1 = only the lower arm/leg (elbow/knee).
AnimationConverter.ARM_BLEND = 0.5
AnimationConverter.LEG_BLEND = 0.5

-- The animation is resampled so no two output keyframes are further apart than this (seconds).
-- Smaller = smoother but bigger files. 1/60 matches the 60 Hz Roblox plays animations at; 0 turns resampling off.
AnimationConverter.SAMPLE_STEP = 1 / 60
local CONSTANT_GAP = 0.001

AnimationConverter.R15_TO_R6 = "r15_to_r6"
AnimationConverter.R6_TO_R15 = "r6_to_r15"

local IDENTITY = CFrame.new()
-- Rotation parts of the default R6 Motor6D C0/C1 (they are the same on both sides of each joint).
local ROOT6 = CFrame.new(0, 0, 0, -1, 0, 0, 0, 0, 1, 0, 1, 0)
local LEFT6 = CFrame.new(0, 0, 0, 0, 0, -1, 0, 1, 0, 1, 0, 0)
local RIGHT6 = CFrame.new(0, 0, 0, 0, 0, 1, 0, 1, 0, -1, 0, 0)

local function joint(child: string, parent: string, c0: CFrame)
	return { child = child, parent = parent, c0 = c0, c1 = c0 }
end

-- Joints are listed parent first. A pose is named after the joint's child part.
local R6 = {
	joints = {
		joint("Torso", "HumanoidRootPart", ROOT6),
		joint("Head", "Torso", ROOT6),
		joint("Left Arm", "Torso", LEFT6),
		joint("Right Arm", "Torso", RIGHT6),
		joint("Left Leg", "Torso", LEFT6),
		joint("Right Leg", "Torso", RIGHT6),
	},
}

local R15 = {
	joints = {
		joint("LowerTorso", "HumanoidRootPart", IDENTITY),
		joint("UpperTorso", "LowerTorso", IDENTITY),
		joint("Head", "UpperTorso", IDENTITY),
		joint("LeftUpperArm", "UpperTorso", IDENTITY),
		joint("LeftLowerArm", "LeftUpperArm", IDENTITY),
		joint("LeftHand", "LeftLowerArm", IDENTITY),
		joint("RightUpperArm", "UpperTorso", IDENTITY),
		joint("RightLowerArm", "RightUpperArm", IDENTITY),
		joint("RightHand", "RightLowerArm", IDENTITY),
		joint("LeftUpperLeg", "LowerTorso", IDENTITY),
		joint("LeftLowerLeg", "LeftUpperLeg", IDENTITY),
		joint("LeftFoot", "LeftLowerLeg", IDENTITY),
		joint("RightUpperLeg", "LowerTorso", IDENTITY),
		joint("RightLowerLeg", "RightUpperLeg", IDENTITY),
		joint("RightFoot", "RightLowerLeg", IDENTITY),
	},
}

-- Which source joints feed each target joint. A target pose only gets weight 1 when at least
-- one of its sources is actually keyed, so partial animations (e.g. an upper-body wave) don't
-- start overriding joints the original never touched.
local CONTRIBUTORS_R15_TO_R6 = {
	Torso = { "LowerTorso", "UpperTorso" },
	Head = { "Head" },
	["Left Arm"] = { "LeftUpperArm", "LeftLowerArm" },
	["Right Arm"] = { "RightUpperArm", "RightLowerArm" },
	["Left Leg"] = { "LeftUpperLeg", "LeftLowerLeg" },
	["Right Leg"] = { "RightUpperLeg", "RightLowerLeg" },
}

local CONTRIBUTORS_R6_TO_R15 = {
	LowerTorso = { "Torso" },
	UpperTorso = { "Torso" },
	Head = { "Head" },
	LeftUpperArm = { "Left Arm" },
	LeftLowerArm = { "Left Arm" },
	LeftHand = { "Left Arm" },
	RightUpperArm = { "Right Arm" },
	RightLowerArm = { "Right Arm" },
	RightHand = { "Right Arm" },
	LeftUpperLeg = { "Left Leg" },
	LeftLowerLeg = { "Left Leg" },
	LeftFoot = { "Left Leg" },
	RightUpperLeg = { "Right Leg" },
	RightLowerLeg = { "Right Leg" },
	RightFoot = { "Right Leg" },
}

local function rotationOf(cframe: CFrame): CFrame
	return cframe - cframe.Position
end

-- poses: name -> CFrame. Returns part name -> world rotation (root at identity).
local function forward(rig, poses)
	local world = { HumanoidRootPart = IDENTITY }
	for _, j in rig.joints do
		local pose = poses[j.child]
		local poseRotation = if pose then rotationOf(pose) else IDENTITY
		world[j.child] = world[j.parent] * j.c0 * poseRotation * j.c1:Inverse()
	end
	return world
end

-- desired: part name -> world rotation (parts left out stay at rest relative to their parent).
-- Returns name -> pose CFrame (rotation only).
local function solve(rig, desired)
	local world = { HumanoidRootPart = IDENTITY }
	local poses = {}
	for _, j in rig.joints do
		local parent = world[j.parent]
		local want = desired[j.child] or (parent * j.c0 * j.c1:Inverse())
		poses[j.child] = j.c0:Inverse() * parent:Inverse() * want * j.c1
		world[j.child] = want
	end
	return poses
end

local function r15ToR6Targets(w)
	local armBlend, legBlend = AnimationConverter.ARM_BLEND, AnimationConverter.LEG_BLEND
	return {
		Torso = w.UpperTorso,
		Head = w.Head,
		["Left Arm"] = w.LeftUpperArm:Lerp(w.LeftLowerArm, armBlend),
		["Right Arm"] = w.RightUpperArm:Lerp(w.RightLowerArm, armBlend),
		["Left Leg"] = w.LeftUpperLeg:Lerp(w.LeftLowerLeg, legBlend),
		["Right Leg"] = w.RightUpperLeg:Lerp(w.RightLowerLeg, legBlend),
	}
end

local function r6ToR15Targets(w)
	return {
		UpperTorso = w.Torso,
		Head = w.Head,
		LeftUpperArm = w["Left Arm"],
		LeftLowerArm = w["Left Arm"],
		LeftHand = w["Left Arm"],
		RightUpperArm = w["Right Arm"],
		RightLowerArm = w["Right Arm"],
		RightHand = w["Right Arm"],
		LeftUpperLeg = w["Left Leg"],
		LeftLowerLeg = w["Left Leg"],
		LeftFoot = w["Left Leg"],
		RightUpperLeg = w["Right Leg"],
		RightLowerLeg = w["Right Leg"],
		RightFoot = w["Right Leg"],
	}
end

local function detectRig(sequence: KeyframeSequence): string?
	for _, keyframe in sequence:GetKeyframes() do
		local hasTorso = false
		for _, descendant in keyframe:GetDescendants() do
			if descendant:IsA("Pose") then
				if descendant.Name == "UpperTorso" or descendant.Name == "LowerTorso" then
					return "R15"
				elseif descendant.Name == "Torso" then
					hasTorso = true
				end
			end
		end
		if hasTorso then
			return "R6"
		end
	end
	return nil
end

-- Roblox plays a pose's easing on the way INTO that pose. We bake it into samples and write
-- everything back as linear, so the result does not depend on how R6 vs R15 interpolate.
local function ease(style: EnumItem, direction: EnumItem, alpha: number): number
	local styleName = style.Name
	if styleName == "Constant" then
		return 0
	elseif styleName == "Linear" then
		return alpha
	end

	local ok, value = pcall(function()
		local tweenStyle = Enum.EasingStyle[if styleName == "CubicV2" then "Cubic" else styleName]
		return TweenService:GetValue(alpha, tweenStyle, Enum.EasingDirection[direction.Name])
	end)
	return if ok then value else alpha
end

-- One track per joint: only poses with weight > 0 are real keys. A joint missing from a
-- keyframe (or keyed at weight 0) keeps moving between its neighbouring keys instead of
-- snapping to the rest pose.
local function buildTracks(sequence: KeyframeSequence)
	local tracks = {}
	local times = {}
	local seenTimes = {}
	local keyframeAt = {}
	local constantTimes = {}

	for _, keyframe in sequence:GetKeyframes() do
		local time = keyframe.Time
		if not seenTimes[time] then
			seenTimes[time] = true
			table.insert(times, time)
		end
		keyframeAt[time] = keyframeAt[time] or keyframe

		for _, descendant in keyframe:GetDescendants() do
			if descendant:IsA("Pose") and descendant.Weight > 0 then
				local track = tracks[descendant.Name]
				if not track then
					track = {}
					tracks[descendant.Name] = track
				end
				table.insert(track, {
					time = time,
					cframe = descendant.CFrame,
					style = descendant.EasingStyle,
					direction = descendant.EasingDirection,
				})
				if descendant.EasingStyle.Name == "Constant" then
					constantTimes[time] = true
				end
			end
		end
	end

	table.sort(times)
	for _, track in tracks do
		table.sort(track, function(a, b)
			return a.time < b.time
		end)
	end

	return tracks, times, keyframeAt, constantTimes
end

local function sampleTrack(track, time: number): CFrame
	local count = #track
	if count == 0 then
		return IDENTITY
	end
	if time <= track[1].time then
		return track[1].cframe
	end
	if time >= track[count].time then
		return track[count].cframe
	end

	local lo, hi = 1, count
	while hi - lo > 1 do
		local mid = (lo + hi) // 2
		if track[mid].time <= time then
			lo = mid
		else
			hi = mid
		end
	end

	local a, b = track[lo], track[hi]
	local alpha = (time - a.time) / (b.time - a.time)
	return a.cframe:Lerp(b.cframe, ease(b.style, b.direction, alpha))
end

-- Every original keyframe time, plus evenly spaced extra samples so no gap is longer than
-- SAMPLE_STEP, plus a sample just before keys that use Constant easing (to keep the hard step).
local function buildSampleTimes(times, constantTimes)
	local step = AnimationConverter.SAMPLE_STEP
	local out = {}

	for i, a in times do
		table.insert(out, a)

		local b = times[i + 1]
		if b then
			local gap = b - a
			if step > 0 and gap > step then
				local extra = math.ceil(gap / step) - 1
				for k = 1, extra do
					table.insert(out, a + gap * k / (extra + 1))
				end
			end
			if constantTimes[b] and gap > 2 * CONSTANT_GAP then
				table.insert(out, b - CONSTANT_GAP)
			end
		end
	end

	return out
end

local function convertSample(time: number, tracks, mode: string, source: Keyframe?): Keyframe
	local fromRig, toRig, targets, contributors
	if mode == AnimationConverter.R15_TO_R6 then
		fromRig, toRig, targets, contributors = R15, R6, r15ToR6Targets, CONTRIBUTORS_R15_TO_R6
	else
		fromRig, toRig, targets, contributors = R6, R15, r6ToR15Targets, CONTRIBUTORS_R6_TO_R15
	end

	local sourceCFrames = {}
	for name, track in tracks do
		sourceCFrames[name] = sampleTrack(track, time)
	end

	local poses = solve(toRig, targets(forward(fromRig, sourceCFrames)))

	-- Carry the root pose's translation over (hip bob, crouch, ...).
	local sourceRoot, targetRoot = fromRig.joints[1], toRig.joints[1]
	local sourceRootPose = sourceCFrames[sourceRoot.child]
	if sourceRootPose then
		local delta = sourceRoot.c0:VectorToWorldSpace(sourceRootPose.Position)
		poses[targetRoot.child] += targetRoot.c0:Inverse():VectorToWorldSpace(delta)
	end

	local out = Instance.new("Keyframe")
	out.Time = time
	if source then
		out.Name = source.Name
		for _, child in source:GetChildren() do
			if child:IsA("KeyframeMarker") then
				child:Clone().Parent = out
			end
		end
	end

	local rootPose = Instance.new("Pose")
	rootPose.Name = "HumanoidRootPart"
	rootPose.CFrame = sourceCFrames.HumanoidRootPart or IDENTITY
	rootPose.Weight = if tracks.HumanoidRootPart then 1 else 0
	rootPose.Parent = out

	local made = { HumanoidRootPart = rootPose }
	for _, j in toRig.joints do
		local pose = Instance.new("Pose")
		pose.Name = j.child
		pose.CFrame = poses[j.child]

		local keyed = false
		for _, sourceName in contributors[j.child] or {} do
			if tracks[sourceName] then
				keyed = true
				break
			end
		end
		pose.Weight = if keyed then 1 else 0

		pose.Parent = made[j.parent]
		made[j.child] = pose
	end

	return out
end

local function convertSequence(sequence: KeyframeSequence, mode: string): KeyframeSequence
	local out = Instance.new("KeyframeSequence")
	out.Name = sequence.Name
	out.Loop = sequence.Loop
	out.Priority = sequence.Priority

	local tracks, times, keyframeAt, constantTimes = buildTracks(sequence)
	for _, time in buildSampleTimes(times, constantTimes) do
		convertSample(time, tracks, mode, keyframeAt[time]).Parent = out
	end

	return out
end

local function findSequence(instances: { Instance }): KeyframeSequence?
	for _, instance in instances do
		if instance:IsA("KeyframeSequence") then
			return instance
		end
	end
	for _, instance in instances do
		local found = instance:FindFirstChildWhichIsA("KeyframeSequence", true)
		if found then
			return found
		end
	end
	return nil
end

-- Returns (converted buffer, "converted") or (nil, "skipped", reason). Errors on unreadable data.
function AnimationConverter.convertBuffer(data: buffer, mode: string): (buffer?, string, string?)
	if mode ~= AnimationConverter.R15_TO_R6 and mode ~= AnimationConverter.R6_TO_R15 then
		error("unknown conversion mode: " .. tostring(mode))
	end

	local instances = SerializationService:DeserializeInstancesAsync(data)

	local function cleanup(extra: Instance?)
		for _, instance in instances do
			instance:Destroy()
		end
		if extra then
			extra:Destroy()
		end
	end

	local sequence = findSequence(instances)
	if not sequence then
		cleanup()
		return nil, "skipped", "not a KeyframeSequence (CurveAnimations are not supported)"
	end

	local rig = detectRig(sequence)
	local wanted = if mode == AnimationConverter.R15_TO_R6 then "R15" else "R6"
	if rig ~= wanted then
		cleanup()
		return nil, "skipped", if rig then `already {rig}, nothing to convert` else "could not tell which rig it is for"
	end

	local ok, converted = pcall(convertSequence, sequence, mode)
	if not ok then
		cleanup()
		error(converted)
	end

	local serialized = SerializationService:SerializeInstancesAsync({ converted })
	cleanup(converted)
	return serialized, "converted", nil
end

return AnimationConverter
