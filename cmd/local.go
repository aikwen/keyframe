package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/urfave/cli/v3"

	"keyframe/internal"
)

type localServer struct {
	display  int
	interval time.Duration
	quality  int

	frame atomic.Pointer[[]byte]
}

var LocalCommand = &cli.Command{
	Name:  "local",
	Usage: "Share screen frames over TCP",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "0.0.0.0",
			Usage: "TCP listen address",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 23333,
			Usage: "TCP listen port",
		},
		&cli.IntFlag{
			Name:  "display",
			Value: 0,
			Usage: "Display index",
		},
		&cli.DurationFlag{
			Name:  "interval",
			Value: 100 * time.Millisecond,
			Usage: "Screen capture interval",
		},
		&cli.IntFlag{
			Name:  "quality",
			Value: 80,
			Usage: "JPEG quality (1-100)",
		},
	},
	Action: runLocal,
}

func runLocal(ctx context.Context, cmd *cli.Command) error {
	port := cmd.Int("port")
	display := cmd.Int("display")
	quality := cmd.Int("quality")
	interval := cmd.Duration("interval")

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if interval <= 0 {
		return fmt.Errorf("invalid interval: %s", interval)
	}
	if quality < 1 || quality > 100 {
		return fmt.Errorf("invalid JPEG quality: %d", quality)
	}

	server := &localServer{
		display:  display,
		interval: interval,
		quality:  quality,
	}

	addr := net.JoinHostPort(cmd.String("host"), fmt.Sprint(port))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer listener.Close()

	slog.Info("local server started",
		"address", listener.Addr().String(),
		"display", display,
		"interval", interval,
		"quality", quality,
	)

	// 初始截图，避免客户端连接后立即收到空帧。
	if err := server.capture(); err != nil {
		slog.Warn("initial capture failed", "error", err)
	}

	go server.captureLoop(ctx)

	// 取消上下文时关闭 Listener，使 Accept 退出。
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			slog.Error("accept failed", "error", err)
			continue
		}

		go server.handleConn(conn)
	}
}

func (s *localServer) captureLoop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if err := s.capture(); err != nil {
				slog.Warn("capture failed", "error", err)
			}
		}
	}
}

func (s *localServer) capture() error {
	img, err := internal.Capture(s.display)
	if err != nil {
		return err
	}

	data, err := internal.EncodeJPEG(img, s.quality)
	if err != nil {
		return err
	}

	// 发布后不再修改 data。
	// 客户端持有旧帧引用时，旧数据依然有效。
	s.frame.Store(&data)

	return nil
}

func (s *localServer) handleConn(conn net.Conn) {
	defer conn.Close()

	addr := conn.RemoteAddr().String()
	slog.Info("client connected", "address", addr)
	defer slog.Info("client disconnected", "address", addr)

	var request [1]byte
	var header [4]byte

	for {
		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return
		}

		if _, err := io.ReadFull(conn, request[:]); err != nil {
			if err != io.EOF {
				slog.Debug("read request failed", "address", addr, "error", err)
			}
			return
		}

		frame := s.frame.Load()

		var data []byte
		if frame != nil {
			data = *frame
		}

		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return
		}

		binary.BigEndian.PutUint32(header[:], uint32(len(data)))

		if err := writeFull(conn, header[:]); err != nil {
			slog.Debug("write header failed", "address", addr, "error", err)
			return
		}

		if len(data) > 0 {
			if err := writeFull(conn, data); err != nil {
				slog.Debug("write frame failed", "address", addr, "error", err)
				return
			}
		}
	}
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}

	return nil
}
