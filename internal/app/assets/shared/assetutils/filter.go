package assetutils

import (
	"github.com/kartFr/Asset-Reuploader/internal/app/context"
	"github.com/kartFr/Asset-Reuploader/internal/app/request"
	"github.com/kartFr/Asset-Reuploader/internal/roblox/develop"
)

func NewFilter(ctx *context.Context, r *request.Request, assetTypeID int32) func(assetsInfo develop.GetAssetsInfoResponse) []*develop.AssetInfo {
	return NewFilterWithOptions(ctx, r, assetTypeID, false)
}

// NewFilterWithOptions is NewFilter, but includeOwned keeps assets the game's creator (or the
// logged in user) already owns. Normally those are skipped because they can be used as they are;
// converting animations needs them reuploaded anyway.
func NewFilterWithOptions(ctx *context.Context, r *request.Request, assetTypeID int32, includeOwned bool) func(assetsInfo develop.GetAssetsInfoResponse) []*develop.AssetInfo {
	creatorID := r.CreatorID
	userID := ctx.Client.UserInfo.ID
	checkUserID := !r.IsGroup

	return func(assetsInfo develop.GetAssetsInfoResponse) []*develop.AssetInfo {
		filteredAssetsInfo := assetsInfo.Data[:0]
		for _, info := range assetsInfo.Data {
			if info.TypeID != assetTypeID {
				continue
			}

			assetCreatorID := info.Creator.TargetID
			if assetCreatorID == 1 {
				continue
			}
			if !includeOwned && (assetCreatorID == creatorID || (checkUserID && assetCreatorID == userID)) {
				continue
			}

			filteredAssetsInfo = append(filteredAssetsInfo, info)
		}
		return filteredAssetsInfo
	}
}
