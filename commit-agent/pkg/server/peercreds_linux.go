/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

//go:build linux

package server

import (
	"context"
	"errors"
	"net"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/credentials"
)

// peerCredsTransport implements credentials.TransportCredentials by reading
// SO_PEERCRED on the accepted Unix socket and surfacing it as a peer's
// AuthInfo. For non-UDS connections it is a no-op (returns empty authInfo).
type peerCredsTransport struct{}

func platformPeerCredentials() credentials.TransportCredentials {
	return peerCredsTransport{}
}

func (peerCredsTransport) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "ucred"}
}

func (peerCredsTransport) Clone() credentials.TransportCredentials {
	return peerCredsTransport{}
}

func (peerCredsTransport) OverrideServerName(string) error { return nil }

// ClientHandshake is unused — commit-agent is server-only — but the
// credentials.TransportCredentials interface requires it.
func (peerCredsTransport) ClientHandshake(_ context.Context, _ string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return conn, peerCredAuthInfo{}, nil
}

func (peerCredsTransport) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return conn, peerCredAuthInfo{}, nil
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return conn, nil, err
	}

	var (
		cred *unix.Ucred
		serr error
	)
	cerr := raw.Control(func(fd uintptr) {
		cred, serr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if cerr != nil {
		return conn, nil, cerr
	}
	if serr != nil {
		return conn, nil, serr
	}
	if cred == nil {
		return conn, nil, errors.New("nil ucred from SO_PEERCRED")
	}
	return conn, peerCredAuthInfo{UID: cred.Uid, GID: cred.Gid, PID: cred.Pid}, nil
}
