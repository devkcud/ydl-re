package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"yoinkdl/internal/queue"
)

func Call(ctx context.Context, paths Paths, op string, args any) (Reply, error) {
	if err := ctx.Err(); err != nil {
		return Reply{}, err
	}
	data, err := json.Marshal(struct {
		Version  int    `json:"version"`
		StateDir string `json:"state_dir"`
		Op       string `json:"op"`
		Args     any    `json:"args"`
	}{Version, paths.State, op, args})
	if err != nil || len(data) >= MaxRequestSize {
		return Reply{}, ErrProtocol
	}
	data = append(data, '\n')
	if _, err := decodeCommand(data, paths.State); err != nil {
		return Reply{}, err
	}
	conn, err := connect(ctx, paths)
	if err != nil {
		return Reply{}, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if err := conn.SetWriteDeadline(time.Now().Add(Timeout)); err != nil {
		return Reply{}, errors.Join(ErrConnection, ctx.Err())
	}
	if err := writeFrame(conn, data); err != nil {
		return Reply{}, callError(ctx, op, ErrConnection)
	}
	if err := conn.SetReadDeadline(time.Now().Add(Timeout)); err != nil {
		return Reply{}, callError(ctx, op, ErrConnection)
	}
	data, err = readFrame(conn, MaxReplySize)
	if err != nil {
		return Reply{}, callError(ctx, op, ErrConnection)
	}
	r, err := decodeReply(data, op)
	if err != nil {
		return Reply{}, callError(ctx, op, err)
	}
	if r.Error != nil {
		return r, r.Error
	}
	return r, nil
}

func connect(ctx context.Context, p Paths) (*net.UnixConn, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if !p.Fallback {
		base, err := os.Lstat(filepath.Dir(p.Runtime))
		if err != nil || !privateDirectory(base) || base.Mode().Perm() != 0o700 {
			return nil, ErrAccess
		}
	}
	file, err := os.OpenFile(p.Runtime, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, ErrConnection
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !privateDirectory(info) {
		return nil, ErrAccess
	}
	path := fmt.Sprintf("/proc/self/fd/%d/%s", file.Fd(), SocketName)
	socket, err := os.Lstat(path)
	if err != nil {
		return nil, ErrConnection
	}
	if !ownedSocket(socket) || socket.Mode().Perm() != 0o600 {
		return nil, ErrAccess
	}
	dial := net.Dialer{Timeout: Timeout}
	raw, err := dial.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, errors.Join(ErrConnection, ctx.Err())
	}
	conn := raw.(*net.UnixConn)
	current, err := os.Lstat(p.Runtime)
	if err != nil || !privateDirectory(current) || !os.SameFile(info, current) {
		conn.Close()
		return nil, ErrChanged
	}
	current, err = os.Lstat(path)
	if err != nil || !ownedSocket(current) || !os.SameFile(socket, current) {
		conn.Close()
		return nil, ErrChanged
	}
	if err := checkPeer(conn, uint32(os.Geteuid())); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func callError(ctx context.Context, op string, err error) error {
	err = errors.Join(err, ctx.Err())
	if op != "ping" && op != "list" {
		err = errors.Join(queue.ErrOutcomeUnknown, err)
	}
	return err
}
