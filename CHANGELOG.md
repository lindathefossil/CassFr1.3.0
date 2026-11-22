# [1.3.0] - CassFr

Animation R15 <-> R6 conversion.

- New toggles in the plugin's Animation tab: "Convert R15 -> R6" and "Convert R6 -> R15" (one at a time). While reuploading, each animation is downloaded as usual (including the place-id route), converted by the plugin, then uploaded.
- Conversion works on the keyframe data (forward kinematics on the source rig, solved onto the target rig), so nothing is posed in the workspace. Based on the idea of Andree's R15 to R6 Animation Converter plugin.
- Animations that are already the target rig, or are not KeyframeSequences (CurveAnimations), are uploaded unchanged and the console says so.
- Converted runs keep their own saved progress, so they never reuse ids from an unconverted run.
- New local endpoints used by the plugin: GET /convert/next and POST /convert/result.
- Keep Studio open with the plugin until the reupload finishes, the plugin does the converting.
- With conversion on, animations you or your group already own are reuploaded too (normally they are skipped because they work as they are).
- R15 -> R6 is approximate: R6 has no elbows, knees, hands or feet. See ARM_BLEND / LEG_BLEND in AnimationConverter.luau.

# [1.2.2] - CassFr

Sound spoofer fixes and credit wording.

- Fixed a crash: if the creator's places could not be loaded, the sound batch kept going with an empty place list and panicked. It now stops and reports the error.
- Fixed a second crash risk when the asset location lookup used for permission checks failed, or returned an empty error list.
- Sounds that no place could deliver are no longer dropped silently. Each one is reported as a failure with the reason Roblox gave (and a hint to add the original game's place id).
- Removed the wrong author credit from the UI footer and README. The LICENSE file is unchanged.

# [1.2.1] - CassFr

Plugin fix for Studio crashing and slow id replacement.

- New background applier in the plugin. Incoming new ids are queued and applied in the background instead of one burst per poll.
- Scripts: every script is now rewritten in a single pass with all of its ids at once (before, each id searched the whole source, one id at a time) and at most about every 3 seconds, plus once at the end. Uses UpdateSourceAsync.
- Animations, sounds and other instances are changed in small time slices spread over frames, so Studio keeps responding.
- The Output window no longer gets a line for every batch. It prints a short summary about every 5 seconds, and only the first few failures.
- The plugin applies everything that was already received before it reports finished, and also when stopped by an error or disconnect.
- Meshes still use the old one-by-one path, which is slow for very large amounts.

# [1.2.0] - CassFr

- New plain web UI with a Settings tab. Presets (Gentle, Balanced, Turbo, Custom), Studio apply speed (1s, 0.5s, 0.25s), saved progress and open-on-start toggles, all saved to config.ini. Carries a "vibecoded, not a web designer" note.
- Saved progress: finished ids are written to cassfr-progress.json and skipped on the next run (not for sounds). Can be cleared in Settings.
- Dashboard shows the current upload speed and how many 429 (slow down) responses Roblox has sent.
- The plugin reads the poll speed from the executable when a reupload starts.
- Settings can only be changed from the dashboard page itself (custom header and origin check), so other websites cannot change them.
- config.ini reads and writes are now thread safe.

# [1.1.0] - CassFr

- Faster. Asset info lookups used a fixed 100 requests a minute and animation uploads a fixed 420 a minute. Both now start higher and adapt: they speed up while Roblox allows it, halve on a 429 and recover afterwards. The learned rate carries over to the next chunk.
- New config.ini keys: `info_rate_start`, `info_rate_max`, `animation_rate_start`, `animation_rate_max`, `animation_concurrency`, `animation_parallel_batch`.
- Asset info requests now understand 429 responses (with Retry-After) and retry up to 6 times instead of 3.
- Sounds and meshes are unchanged.

# [1.0.0] - CassFr

CassFr is a fork of BacFr (1.7.x).

## Executable and plugin

- Removed the 1024 KB size limit. The plugin now splits the ids into chunks that fit under Roblox's request limit and sends them one at a time. Each chunk is reuploaded as fast as it can, then the next one starts automatically.
- Added a live dashboard at `http://localhost:<port>/ui` with overall progress, per-chunk progress, stats and a live log. Opens automatically (`open_dashboard=false` in `config.ini` turns that off).
- Plugin title bar shows the current chunk while reuploading.
- The executable prints which chunk it is on and the dashboard link.
- `Export json when done` now writes a single file across all chunks and is written at most every 2 seconds instead of after every asset.
- Fixed pending id replacements being dropped if they arrived while the plugin was polling.
- Opening the port in a browser no longer consumes pending replacements, and `/favicon.ico` style requests no longer reach the polling endpoint.
- The plugin reports `failed` starts instead of hanging, and no longer checks the upstream plugin for updates.
- Renamed the executable and release files to CassFr.

# [1.4.1](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.4.1) - September 15th, 2025
- Got rid of atomic writing (very naive to put that in lmaooo)
- (actually) Fixed mesh changing colors.
- Fixed case in animations where if the owner had a place it wouldn't look at the place list. (oops)

Okay pls read this...
Spoofing has to be different now, due to roblox updates its no longer as simple as getting it from thin air.
We now HAVE to provide a place id with the animation, which in some cases is fine if its from a public game.
If its from a private game you guys will have to provide the ID under filter. 
Keep in mind that asset permission is based off universe the universe. But the api itself is based off places, so you only have to provide 1 place from that universe.

# [1.4.0](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.4.0) - August 6th, 2025

- Changed it so writing to files is now atomic. ([#58](https://github.com/kartFr/Asset-Reuploader/pull/58))
- Fixed saving cookie for mac users. ([#27](https://github.com/kartFr/Asset-Reuploader/pull/27))
- Fixed reupload filter not taking into account whether it was reuploading to a group or user. ([#51](https://github.com/kartFr/Asset-Reuploader/pull/51))
- Fixed rate limit window going into the negatives when resetting. ([#53](https://github.com/kartFr/Asset-Reuploader/pull/53))
- Fixed default config not getting set if it failed to read the config file. ([#57](https://github.com/kartFr/Asset-Reuploader/pull/57))
- Fixed mesh changing colors. ([#71](https://github.com/kartFr/Asset-Reuploader/pull/71))
- Fixed animation spoofing and made place id input a global setting. ([#73](https://github.com/kartFr/Asset-Reuploader/pull/73))

Place ids do not affect mesh ids yet, hopefully this is fine.

### Accessibility changes ([#46](https://github.com/kartFr/Asset-Reuploader/pull/46))

- Added port to startup message.
- Added message to tell user you can rerun without restarting when finished reuploading.
- Added a + button to text input to make it more accessible.
- Changed default port from `61048` to `38073`
- Changed valid port range from `2049-65535` to `1024-49151`
- Fixed stats displaying on failure to start reuploading.
- Fixed notification header being tiny.

### Mesh fixes ([#55](https://github.com/kartFr/Asset-Reuploader/pull/55))

- Changed error handling, should properly handle errors for transferring attributes, tags, and children.
- Fixed sandbox not enabled error message.
- Fixed reuploading meshes not changing joints. (this will be laggy for now, performance improvements will come in v2.0.0 as code needs a big revamp in some areas for this)
- Fixed reuploading trying to reparent touch interests.

# [1.3.1](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.3.1) - April 27th, 2025

- Changed client timeout from `15s` to `30s` ([#24](https://github.com/kartFr/Asset-Reuploader/pull/24))

# [1.3.0](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.3.0) - April 27th, 2025

- Added a changelog. Adds changelog to releases ([#16](https://github.com/kartFr/Asset-Reuploader/pull/16))

## Executable

- Added fixed window limiter instead of naively sleeping. (big change very good for reuploading 😇) ([#18](https://github.com/kartFr/Asset-Reuploader/pull/18))
- Fixed `ErrNoCreateItemPermission` saying `permissios` instead of `permission`. ([#19](https://github.com/kartFr/Asset-Reuploader/pull/19))
- Fixed blank error messages ([#21](https://github.com/kartFr/Asset-Reuploader/pull/21))

## Plugin 1.2.1

- Changed audio tip to be more clear ([#10](https://github.com/kartFr/Asset-Reuploader/pull/10))
- Fixed tip bar theme not updating

# [1.2.0](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.2.0) - April 18th, 2025

## Executable

- Changed client context timeout from `10s` to `15s`
- Added ratelimit to reuploading animations. ([#5](https://github.com/kartFr/Asset-Reuploader/pull/5))
- Added micro optimizations that nobody will ever notice.

## Plugin 1.2.0

- Added notification border
- Some other stuff nobody will ever care about (me fr)
- lmk if you read this :p

### Theme changes

- Changed `Foreground` to `MainBackground`
- Added Background image
- Added Text size

# [1.1.0](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.1.0) - April 16th, 2025

## Executable

- Added `Unauthorized` status on sound and mesh reupload requests to provide proper warnings for the plugin.
- Added an editable configuration file.
- Changed client context timeout increased `5s-10s`
- Changed group error messages to be clearer.
- Fixed issue where reuploading wasn't happening in goroutines.

## Plugin 1.1.0

- Changed IDs to replace on new threads.
- Changed Input element to it's own class.
- Added preloading for theme assets.
- Fixed reuploaded selected using filter options. ([#3](https://github.com/kartFr/Asset-Reuploader/pull/3))

# [1.0.0](https://github.com/kartFr/Asset-Reuploader/releases/tag/1.0.0) - April 12th, 2025

Initial release
