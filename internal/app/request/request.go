package request

import (
	"github.com/kartFr/Asset-Reuploader/internal/app/convert"
	"github.com/kartFr/Asset-Reuploader/internal/roblox"
	"github.com/kartFr/Asset-Reuploader/internal/roblox/games"
)

type RawRequest struct {
	PlaceID         int64   `json:"placeId"`
	CreatorID       int64   `json:"creatorId"`
	IDs             []int64 `json:"ids"`
	DefaultPlaceIDs []int64 `json:"defaultPlaceIds"`
	PluginVersion   string  `json:"pluginVersion"`
	AssetType       string  `json:"assetType"`
	ExportJSON      bool    `json:"exportJSON"`
	IsGroup         bool    `json:"isGroup"`
	ChunkIndex      int     `json:"chunkIndex"`
	ChunkCount      int     `json:"chunkCount"`
	ChunkSizes      []int   `json:"chunkSizes"`
	TotalIDs        int     `json:"totalIds"`
	AnimationConvert string `json:"animationConvert"`
}

type Request struct {
	UniverseID      int64
	PlaceID         int64
	CreatorID       int64
	IDs             []int64
	DefaultPlaceIDs []int64
	IsGroup         bool

	// AnimationConvert is "", convert.ModeR15ToR6 or convert.ModeR6ToR15.
	AnimationConvert string
}

func FromRawRequest(c *roblox.Client, req *RawRequest) (*Request, error) {
	placeID := req.PlaceID

	placesInfo, err := games.MultiGetPlaceDetails(c, []int64{placeID})
	if err != nil {
		return nil, err
	}

	animationConvert := ""
	if req.AssetType == "Animation" && convert.ValidMode(req.AnimationConvert) {
		animationConvert = req.AnimationConvert
	}

	return &Request{
		UniverseID:      placesInfo[0].UniverseID,
		PlaceID:         placeID,
		CreatorID:       req.CreatorID,
		IDs:             req.IDs,
		DefaultPlaceIDs: req.DefaultPlaceIDs,
		IsGroup:         req.IsGroup,

		AnimationConvert: animationConvert,
	}, nil
}
