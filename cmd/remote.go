package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/urfave/cli/v3"

	"keyframe/internal"
)

type remoteFrame struct {
	Seq  uint64
	Data []byte
}

type remoteClient struct {
	display   int
	interval  time.Duration
	threshold int
	quality   int
	scale     float64
	address   string
	token     string

	client *http.Client
	frame  atomic.Pointer[remoteFrame]
	notify chan struct{}
}

var RemoteCommand = &cli.Command{
	Name:  "remote",
	Usage: "Upload screen frames to a remote server",
	Arguments: []cli.Argument{
		&cli.StringArg{
			Name:     "address",
			Required: true,
		},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "token",
			Usage:    "Authentication token",
			Sources:  cli.EnvVars("KEYFRAME_TOKEN"),
			Required: true,
		},
		&cli.IntFlag{
			Name:  "display",
			Value: 0,
			Usage: "Display index",
		},
		&cli.DurationFlag{
			Name:  "interval",
			Value: 200 * time.Millisecond,
			Usage: "Mouse polling interval",
		},
		&cli.IntFlag{
			Name:  "threshold",
			Value: 20,
			Usage: "Screen edge threshold in pixels",
		},
		&cli.IntFlag{
			Name:  "quality",
			Value: 80,
			Usage: "JPEG quality (1-100)",
		},
		&cli.Float64Flag{
			Name:  "scale",
			Value: 0.75,
			Usage: "Image scale factor (0 < scale <= 1)",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: 20 * time.Second,
			Usage: "HTTP upload timeout",
		},
	},
	Action: runRemote,
}

func runRemote(ctx context.Context, cmd *cli.Command) error {
	address := cmd.StringArg("address")

	u, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") ||
		u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid server URL: %s", address)
	}

	if cmd.Duration("interval") <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if cmd.Int("threshold") <= 0 {
		return fmt.Errorf("threshold must be positive")
	}
	if q := cmd.Int("quality"); q < 1 || q > 100 {
		return fmt.Errorf("invalid JPEG quality: %d", q)
	}
	if s := cmd.Float64("scale"); math.IsNaN(s) || s <= 0 || s > 1 {
		return fmt.Errorf("scale must be in (0, 1]")
	}
	if cmd.Duration("timeout") <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if cmd.Int("display") < 0 {
		return fmt.Errorf("invalid display index")
	}

	endpoint, err := url.JoinPath(address, "frames")
	if err != nil {
		return fmt.Errorf("build upload URL: %w", err)
	}

	r := &remoteClient{
		display:   cmd.Int("display"),
		interval:  cmd.Duration("interval"),
		threshold: cmd.Int("threshold"),
		quality:   cmd.Int("quality"),
		scale:     cmd.Float64("scale"),
		address:   endpoint,
		token:     cmd.String("token"),
		client: &http.Client{
			Timeout: cmd.Duration("timeout"),
		},
		notify: make(chan struct{}, 1),
	}

	slog.Info("remote started",
		"address", r.address,
		"display", r.display,
		"interval", r.interval,
		"threshold", r.threshold,
		"quality", r.quality,
		"scale", r.scale,
	)

	go r.uploadLoop(ctx)

	return r.captureLoop(ctx)
}

func (r *remoteClient) captureLoop(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	var seq uint64

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			atEdge, err := internal.IsCursorAtEdge(
				r.display, r.threshold,
			)
			if err != nil {
				slog.Warn("cursor detection failed", "error", err)
				continue
			}
			if !atEdge {
				continue
			}

			img, err := internal.Capture(r.display)
			if err != nil {
				slog.Warn("capture failed", "error", err)
				continue
			}

			img, err = internal.ScaleImage(img, r.scale)
			if err != nil {
				slog.Warn("scale image failed", "error", err)
				continue
			}

			data, err := internal.EncodeJPEG(img, r.quality)
			if err != nil {
				slog.Warn("JPEG encoding failed", "error", err)
				continue
			}

			seq++
			r.frame.Store(&remoteFrame{
				Seq:  seq,
				Data: data,
			})

			// 只通知有新帧，不积压图片。
			select {
			case r.notify <- struct{}{}:
			default:
			}

			slog.Debug("frame captured",
				"seq", seq,
				"bytes", len(data),
			)
		}
	}
}

func (r *remoteClient) uploadLoop(ctx context.Context) {
	var lastSeq uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.notify:
		}

		for {
			frame := r.frame.Load()
			if frame == nil || frame.Seq <= lastSeq {
				break
			}

			// 当前帧只尝试上传一次。
			lastSeq = frame.Seq

			err := r.upload(ctx, frame)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("frame upload failed",
					"seq", frame.Seq,
					"error", err,
				)
			} else {
				slog.Info("frame uploaded",
					"seq", frame.Seq,
					"bytes", len(frame.Data),
				)
			}

			// 上传结束后重新 Load，
			// 如果期间产生了新帧，直接发送最新帧。
		}
	}
}

func (r *remoteClient) upload(
	ctx context.Context,
	frame *remoteFrame,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.address,
		bytes.NewReader(frame.Data),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("X-Frame-Seq", fmt.Sprint(frame.Seq))

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 少量读取响应体，方便复用 HTTP 连接。
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	return nil
}
