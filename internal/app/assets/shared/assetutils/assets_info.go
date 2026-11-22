package assetutils

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/kartFr/Asset-Reuploader/internal/app/assets/shared/clientutils"
	"github.com/kartFr/Asset-Reuploader/internal/app/config"
	"github.com/kartFr/Asset-Reuploader/internal/app/context"
	"github.com/kartFr/Asset-Reuploader/internal/app/progress"
	"github.com/kartFr/Asset-Reuploader/internal/app/request"
	"github.com/kartFr/Asset-Reuploader/internal/retry"
	"github.com/kartFr/Asset-Reuploader/internal/roblox/develop"
	"github.com/kartFr/Asset-Reuploader/internal/taskqueue"
)

const AssetsInfoChunkSize int = 50

var learnedInfoRate atomic.Int64

type AssetsInfoResult = taskqueue.TaskResult[develop.GetAssetsInfoResponse]

func GetAssetsInfoInChunks(ctx *context.Context, r *request.Request) []chan AssetsInfoResult {
	maxRate := config.GetInt("info_rate_max", 900)
	startRate := min(config.GetInt("info_rate_start", 300), maxRate)
	if learned := int(learnedInfoRate.Load()); learned > 0 {
		startRate = min(max(learned, 60), maxRate)
	}
	queue := taskqueue.NewSmoothQueue[develop.GetAssetsInfoResponse](32, startRate)
	controller := taskqueue.NewRateController(queue.Limiter, startRate, 60, maxRate)

	newAssetsInfoHandler := func(ids []int64) func() (develop.GetAssetsInfoResponse, error) {
		return func() (develop.GetAssetsInfoResponse, error) {
			handler, err := develop.NewAssetsInfoHandler(ctx.Client, ids)
			if err != nil {
				return develop.GetAssetsInfoResponse{}, err
			}

			return retry.Do(
				retry.NewOptions(retry.Tries(6)),
				func(try int) (develop.GetAssetsInfoResponse, error) {
					ctx.PauseController.WaitIfPaused()
					if try > 1 {
						queue.Limiter.Wait()
					}

					assetsInfo, err := handler()
					if err == nil {
						controller.Success()
						progress.Global.SetInfoRate(controller.Rate())
						learnedInfoRate.Store(int64(controller.Rate()))
						return assetsInfo, nil
					}

					var rateLimit *develop.RateLimitError
					if errors.As(err, &rateLimit) {
						progress.Global.AddRateLimit()
						if controller.RateLimited() {
							queue.Chill(rateLimit.RetryAfter)
						}
					} else if err == develop.GetAssetsInfoErrors.ErrUnauthorized {
						clientutils.GetNewCookie(ctx, r, "cookie expired")
					} else {
						switch err.(type) {
						case *net.OpError, *net.DNSError:
							queue.Limiter.Decrement()
						}
					}

					return develop.GetAssetsInfoResponse{}, &retry.ContinueRetry{Err: err}
				},
			)
		}
	}

	ids := r.IDs

	chunkAmount := (len(ids) + AssetsInfoChunkSize - 1) / AssetsInfoChunkSize
	tasks := make([]chan AssetsInfoResult, 0, chunkAmount)
	for start, end := 0, 50; start < len(ids); start, end = start+50, end+50 {
		end = min(end, len(ids))
		idChunk := ids[start:end]
		tasks = append(tasks, queue.QueueTask(newAssetsInfoHandler(idChunk)))
	}

	return tasks
}
