# CassFr

An open-source Roblox asset reuploader. CassFr is a fork of the open-source BacFr / Asset-Reuploader project, with the upload size limit removed and a live progress dashboard added.

It is a Roblox Studio plugin plus a small Go program that runs on your computer. Intended to reupload assets since you are unable to use assets if another account owns them.

## What CassFr changes

**No more size limit.** Roblox only lets a plugin send about 1024 KB per request, so BacFr failed with `Post data too large. Limit: 1024 KB` on big places. CassFr splits the ids into chunks that each fit under the limit and sends them one at a time:

1. The plugin sends chunk 1 and the program reuploads it as fast as it can.
2. When chunk 1 is done and the new ids are written back into your place, the plugin sends chunk 2.
3. This repeats until every chunk is done. Small places still go out as a single chunk, exactly like before.

**Live dashboard.** While reuploading, open `http://localhost:38073/ui` (it opens automatically the first time, turn that off with `open_dashboard=false` in `config.ini`). It shows overall progress, a segmented bar with one segment per chunk, how many assets were reuploaded, elapsed time, an estimate of what is left, and a live log with an errors-only filter. The plugin title bar also shows `chunk 2/5` while it works.

**Faster animations and metadata (1.1.0).** BacFr paced everything at fixed speeds: asset info lookups at 100 requests a minute and animation uploads at 420 a minute. For about 51,000 ids that alone meant roughly 10 minutes just to look the assets up. CassFr now starts faster and adapts. It speeds up while Roblox is happy, cuts the rate in half the moment Roblox answers 429 (rate limited), and then climbs again. It also remembers the rate it learned for the next chunk. You can tune it in `config.ini`:

| key | default | what it does |
| --- | --- | --- |
| `info_rate_start` / `info_rate_max` | 300 / 900 | asset info requests per minute (50 ids each) |
| `animation_rate_start` / `animation_rate_max` | 600 / 1500 | animation upload starts per minute |
| `animation_concurrency` | 48 | uploads in flight at once |
| `animation_parallel_batch` | 10 | batches of 50 ids processed at once |

Lower the max values if you see lots of rate limit messages. Sounds and meshes keep their original fixed limits for now.

**Settings page and saved progress (1.2.0).** The dashboard is now a plain page (it says so on the page, it was vibecoded) with a Progress tab and a Settings tab. In Settings you pick a speed preset (Gentle, Balanced, Turbo or Custom), choose how often Studio checks for new ids (Normal, Fast or Instant), and turn saved progress on or off. Everything is saved to `config.ini` and applies from the next reupload. New ids are written into your place while the rest is still uploading, so "faster" just means the plugin checks more often.

Saved progress writes every finished old to new id to `cassfr-progress.json`. If Studio crashes or you stop halfway, the next run applies those ids straight away and only uploads what is left. It is keyed by asset type and creator, and it is not used for sounds. Clear it from Settings if you want everything uploaded again.

The dashboard also shows the current upload speed and how many times Roblox said slow down (429). If that number keeps climbing, Roblox is the limit, not CassFr, and a faster preset will not help.

**Studio stays responsive (1.2.1).** New ids are queued and applied in the background. Scripts are rewritten once with all their ids, not once per id, and other instances are changed a little each frame. That fixes slow replacing and Studio crashes on big places.

**Smaller fixes**

- Opening the port in a browser no longer eats your pending id replacements.
- Replacements that arrive while the plugin is polling can no longer be dropped.
- `Export json when done` writes one file for the whole run, even across chunks, and no longer rewrites the file on every single asset.
- The plugin no longer nags about being out of date against the upstream plugin.

## Setup

1. Install the plugin: build `plugin/` with [Rojo](https://rojo.space) (`rojo build plugin --output CassFr.rbxm`) and put the `.rbxm` in your Roblox plugins folder, or download `CassFr1.2.2.rbxm` from the releases page.
2. Run `CassFr.exe` (or build it, see below). Paste your `.ROBLOSECURITY` cookie when asked, and an Open Cloud API key for meshes and animations.
3. In Studio open the CassFr plugin, pick Animation, Sound or Mesh, and reupload.

The port is `38073` by default. If you change it in `config.ini` change it in the plugin settings too.

## Building

```
go build -ldflags="-X main.CompatiblePluginVersion=1.2.2 -X main.Version=1.2.2" -o CassFr.exe ./cmd/...
rojo build plugin --output CassFr.rbxm
```

Go 1.23 or newer. The plugin version in `plugin/Version.txt` must match `CompatiblePluginVersion`.

## Currently supported assets

- Animations
- Audio
- Meshes

## Contributing

Pull requests and forks are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License and credits

CassFr is free software under the [GPL-3.0](LICENSE) license. The original BacFr code is also GPL-3.0 and its copyright notice is kept in the LICENSE file. Changes in CassFr are released under the same license, so forks of CassFr must stay open source too.


## Converting animations between R15 and R6

In the plugin's Animation tab, turn on **Convert R15 -> R6** or **Convert R6 -> R15** before reuploading. Each animation is downloaded, converted by the plugin (Studio must stay open until the run finishes), then uploaded. Animations already on the target rig are uploaded unchanged. R15 -> R6 is approximate because R6 has no elbows, knees, hands or feet; tweak `ARM_BLEND` / `LEG_BLEND` in `plugin/src/App/AnimationConverter.luau` (0 = shoulder/hip only, 1 = elbow/knee only).
